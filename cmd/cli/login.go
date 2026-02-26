package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ────────────────────────────────────────────────
// GitHub Device Flow helpers
// ────────────────────────────────────────────────

const (
	githubDeviceCodeURL = "https://github.com/login/device/code"
	githubTokenURL      = "https://github.com/login/oauth/access_token"
	githubUserURL       = "https://api.github.com/user"
)

type githubDeviceCodeResp struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Error           string `json:"error"`
}

type githubTokenResp struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

type githubUserResp struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"` // may be empty if user keeps it private
	Name  string `json:"name"`
}

func githubRequestDeviceCode(clientID string) (*githubDeviceCodeResp, error) {
	resp, err := http.PostForm(githubDeviceCodeURL, url.Values{
		"client_id": {clientID},
		"scope":     {"read:user,user:email"},
	})
	if err != nil {
		return nil, fmt.Errorf("requesting device code: %w", err)
	}
	defer resp.Body.Close()

	var result githubDeviceCodeResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding device code response: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("GitHub error: %s", result.Error)
	}
	return &result, nil
}

func githubPollForToken(clientID, deviceCode string, intervalSec, expiresSec int) (string, error) {
	interval := time.Duration(intervalSec) * time.Second
	deadline := time.Now().Add(time.Duration(expiresSec) * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(interval)

		resp, err := http.PostForm(githubTokenURL, url.Values{
			"client_id":   {clientID},
			"device_code": {deviceCode},
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		})
		if err != nil {
			return "", fmt.Errorf("polling for token: %w", err)
		}

		var result githubTokenResp
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("decoding token response: %w", err)
		}
		resp.Body.Close()

		switch result.Error {
		case "":
			return result.AccessToken, nil
		case "authorization_pending":
			// Still waiting — keep polling
			continue
		case "slow_down":
			// GitHub asks us to back off; add 5s
			interval += 5 * time.Second
			continue
		case "expired_token":
			return "", fmt.Errorf("device code expired — please run 'jennah login' again")
		case "access_denied":
			return "", fmt.Errorf("authorization denied by user")
		default:
			return "", fmt.Errorf("GitHub token error: %s — %s", result.Error, result.ErrorDesc)
		}
	}
	return "", fmt.Errorf("timed out waiting for GitHub authorization")
}

func githubGetUser(accessToken string) (*githubUserResp, error) {
	req, err := http.NewRequest("GET", githubUserURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching GitHub user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub user API returned %d", resp.StatusCode)
	}

	var user githubUserResp
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decoding user response: %w", err)
	}
	return &user, nil
}

// Config holds saved credentials.
type Config struct {
	Email    string `json:"email"`
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id,omitempty"`
	Provider string `json:"provider,omitempty"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "jennah", "config.json"), nil
}

func loadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func prompt(label string) (string, error) {
	fmt.Printf("%s: ", label)
	reader := bufio.NewReader(os.Stdin)
	val, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(val), nil
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to Jennah via GitHub",
	Long:  "jennah login\n\nAuthenticates via GitHub Device Flow. Opens GitHub in your browser so you\ncan authorize without typing passwords or tokens.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Block if already logged in.
		existing, err := loadConfig()
		if err != nil {
			return err
		}
		if existing != nil {
			fmt.Printf("Already logged in as \033[36m%s\033[0m.\n", existing.UserID)
			fmt.Println("Run 'jennah logout' first before logging in again.")
			return nil
		}

		// Require GITHUB_CLIENT_ID to be set.
		clientID := os.Getenv("GITHUB_CLIENT_ID")
		if clientID == "" {
			return fmt.Errorf(
				"GITHUB_CLIENT_ID environment variable is not set.\n" +
					"Create a GitHub OAuth App at https://github.com/settings/developers,\n" +
					"enable Device Flow, and export its Client ID:\n\n" +
					"  export GITHUB_CLIENT_ID=<your-client-id>")
		}

		fmt.Println("Log in to Jennah")
		fmt.Println("────────────────")
		fmt.Println("Authenticating via GitHub...")
		fmt.Println()

		// Step 1: Request a device + user code from GitHub.
		dcResp, err := githubRequestDeviceCode(clientID)
		if err != nil {
			return err
		}

		// Step 2: Ask the user to authorize in their browser.
		fmt.Printf("Open this URL in your browser:\n\n  \033[36m%s\033[0m\n\n", dcResp.VerificationURI)
		fmt.Printf("Then enter this code: \033[1;33m%s\033[0m\n\n", dcResp.UserCode)
		fmt.Printf("Waiting for authorization (expires in %ds)...\n", dcResp.ExpiresIn)

		// Step 3: Poll until approved or expired.
		accessToken, err := githubPollForToken(clientID, dcResp.DeviceCode, dcResp.Interval, dcResp.ExpiresIn)
		if err != nil {
			return err
		}

		// Step 4: Fetch the GitHub user profile.
		ghUser, err := githubGetUser(accessToken)
		if err != nil {
			return fmt.Errorf("failed to fetch GitHub user info: %w", err)
		}

		email := ghUser.Email
		if email == "" {
			// GitHub keeps email private for some users — fall back to a noreply address.
			email = fmt.Sprintf("%d+%s@users.noreply.github.com", ghUser.ID, ghUser.Login)
		}
		userID := strconv.FormatInt(ghUser.ID, 10)

		// Temporarily save config so newGatewayClient can read headers.
		cfg := &Config{Email: email, UserID: userID, Provider: "github"}
		if err := saveConfig(cfg); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}

		fmt.Println()
		fmt.Println("Checking account...")

		gw, err := newGatewayClient(cmd)
		if err != nil {
			path, _ := configPath()
			os.Remove(path)
			return err
		}

		var tenantResult struct {
			TenantID  string `json:"tenantId"`
			UserEmail string `json:"userEmail"`
			CreatedAt string `json:"createdAt"`
		}
		if err := gw.post("/jennah.v1.DeploymentService/GetCurrentTenant", map[string]interface{}{}, &tenantResult); err != nil {
			path, _ := configPath()
			os.Remove(path)
			return fmt.Errorf("could not reach server: %w", err)
		}

		// Determine if this is a brand-new account.
		isNew := false
		if t, parseErr := time.Parse(time.RFC3339, tenantResult.CreatedAt); parseErr == nil {
			isNew = time.Since(t) < 5*time.Second
		}

		if isNew {
			fmt.Println("✅ New account created.")
		} else {
			fmt.Println("✅ Existing account found.")
		}

		// Save tenant ID into config.
		cfg.TenantID = tenantResult.TenantID
		if err := saveConfig(cfg); err != nil {
			return fmt.Errorf("failed to save tenant id: %w", err)
		}

		fmt.Println()
		fmt.Printf("Logged in as \033[36m%s\033[0m (%s)\n", ghUser.Login, email)
		fmt.Println()
		rootCmd.Help()
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of Jennah",
	Long:  "jennah logout\n\nRemoves your locally saved credentials.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		if cfg == nil {
			fmt.Println("Not logged in.")
			return nil
		}

		path, err := configPath()
		if err != nil {
			return err
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}

		fmt.Println("✅ Logged out successfully.")
		return nil
	},
}
