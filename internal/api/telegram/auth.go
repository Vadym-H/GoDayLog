package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidAuth = errors.New("invalid telegram auth")
var ErrAuthExpired = errors.New("auth data expired")

// ValidateMiniApp verifies initData from window.Telegram.WebApp.initData.
// Returns the Telegram user ID string.
func ValidateMiniApp(initData, botToken string) (string, error) {
	vals, err := url.ParseQuery(initData)
	if err != nil {
		return "", fmt.Errorf("%w: parse init_data", ErrInvalidAuth)
	}

	hash := vals.Get("hash")
	if hash == "" {
		return "", fmt.Errorf("%w: missing hash", ErrInvalidAuth)
	}

	var pairs []string
	for k, vs := range vals {
		if k == "hash" {
			continue
		}
		pairs = append(pairs, k+"="+vs[0])
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	// secret_key = HMAC-SHA256(key="WebAppData", message=botToken)
	mac := hmac.New(sha256.New, []byte("WebAppData"))
	mac.Write([]byte(botToken))
	secretKey := mac.Sum(nil)

	mac2 := hmac.New(sha256.New, secretKey)
	mac2.Write([]byte(dataCheckString))
	expected := hex.EncodeToString(mac2.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(hash)) {
		return "", ErrInvalidAuth
	}

	if err := checkAuthDate(vals.Get("auth_date")); err != nil {
		return "", err
	}

	userJSON := vals.Get("user")
	if userJSON == "" {
		return "", fmt.Errorf("%w: missing user field", ErrInvalidAuth)
	}
	var user struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(userJSON), &user); err != nil {
		return "", fmt.Errorf("%w: parse user json", ErrInvalidAuth)
	}
	if user.ID == 0 {
		return "", fmt.Errorf("%w: missing user id", ErrInvalidAuth)
	}

	return strconv.FormatInt(user.ID, 10), nil
}

// ValidateWidget verifies Telegram Login Widget callback params.
// Returns the Telegram user ID string.
func ValidateWidget(params url.Values, botToken string) (string, error) {
	hash := params.Get("hash")
	if hash == "" {
		return "", fmt.Errorf("%w: missing hash", ErrInvalidAuth)
	}

	var pairs []string
	for k, vs := range params {
		if k == "hash" {
			continue
		}
		pairs = append(pairs, k+"="+vs[0])
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	// secret_key = SHA256(botToken) — differs from Mini App
	h := sha256.Sum256([]byte(botToken))
	secretKey := h[:]

	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(dataCheckString))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(hash)) {
		return "", ErrInvalidAuth
	}

	if err := checkAuthDate(params.Get("auth_date")); err != nil {
		return "", err
	}

	userID := params.Get("id")
	if userID == "" {
		return "", fmt.Errorf("%w: missing id param", ErrInvalidAuth)
	}

	return userID, nil
}

func checkAuthDate(authDateStr string) error {
	if authDateStr == "" {
		return fmt.Errorf("%w: missing auth_date", ErrInvalidAuth)
	}
	ts, err := strconv.ParseInt(authDateStr, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: invalid auth_date", ErrInvalidAuth)
	}

	const allowedFutureSkew = time.Minute

	authTime := time.Unix(ts, 0)
	now := time.Now()
	if authTime.After(now.Add(allowedFutureSkew)) {
		return fmt.Errorf("%w: auth_date is in the future", ErrInvalidAuth)
	}
	if now.Sub(authTime) > 24*time.Hour {
		return ErrAuthExpired
	}
	return nil
}
