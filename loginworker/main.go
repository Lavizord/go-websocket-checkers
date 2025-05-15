package main

import (
	"crypto/rand"
	"crypto/tls"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/Lavizord/checkers-server/config"
	"github.com/Lavizord/checkers-server/postgrescli"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/rs/cors"
)

type App struct {
	DB          *postgrescli.PostgresCli
	Throttle    map[string]time.Time
	EmailSender *EmailSender
}

type EmailSender struct {
	From     string
	Password string
}

// Request structures
type LoginRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type VerifyRequest struct {
	Email string `json:"email" validate:"required,email"`
	Code  string `json:"code" validate:"required,len=8"`
}

// Response structure
type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Token   string `json:"token"`
}

var jwtKey = []byte("your-secret-key")

var htmlTemplate = `<!DOCTYPE html>
<html lang="en" style="margin: 0; padding: 0">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Retromind games OTP</title>
    <style>
      html,
      body {
        margin: 0;
        padding: 0;
        height: 100%;
      }
      body {
        font-family: Arial, sans-serif;
        background-color: #222;
        margin: 0;
        padding: 0;
        -webkit-text-size-adjust: 100%;
        -ms-text-size-adjust: 100%;
      }
      .container {
        background-color: #222;
        max-width: 600px;
        margin: 40px auto;
        border-radius: 8px;
        box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
        padding: 30px 40px;
        color: #fff;
      }
      .logo {
        margin: 32px;
      }
      h1 {
        font-size: 24px;
        margin-bottom: 24px;
        color: #fff;
      }
      p {
        font-size: 16px;
        line-height: 1.5;
        margin: 0 0 24px 0;
      }
      .otp-code {
        font-size: 28px;
        font-weight: bold;
        color: #74ff97;
        letter-spacing: 6px;
        text-align: center;
        margin: 24px 0;
        background-color: #000;
        padding: 12px 0;
        border-radius: 6px;
        user-select: all;
      }
      .footer {
        font-size: 14px;
        color: #888888;
        text-align: center;
        margin-top: 36px;
      }
      @media only screen and (max-width: 620px) {
        .container {
          margin: 20px;
          padding: 20px;
        }
        h1 {
          font-size: 20px;
        }
        .otp-code {
          font-size: 24px;
          letter-spacing: 4px;
        }
      }
    </style>
  </head>
  <body>
    <table
      role="presentation"
      width="100%"
      height="100%"
      cellpadding="0"
      cellspacing="0"
      border="0"
      style="width: 100%; height: 100%; background-color: #333"
    >
      <tr>
        <td align="center" valign="middle">
          <div class="container" role="main">
            <img class="logo" width="200" src=" https://s3.eu-central-1.amazonaws.com/play.retromindgames.pt/assets/images/Horizontal/Dual/2.png" />
            <h1>Hi {email},</h1>
            <p>
              Please use the following One-Time Password (OTP) to complete your
              verification process.
            </p>
            <div class="otp-code" aria-label="Your OTP code">XXXXXX</div>
            <p>If you did not request this code, please ignore this email.</p>
            <div class="footer">
              &copy; 2025 Retromind Games. All rights reserved.
            </div>
          </div>
        </td>
      </tr>
    </table>
  </body>
</html>`

func generateToken(email string, hours int) (string, error) {
	claims := jwt.MapClaims{
		"email": email,
		"exp":   time.Now().Add(time.Duration(hours) * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

func (e *EmailSender) SendEmail(to, code string) error {
	server := "smtp.gmail.com"
	port := "465"
	auth := smtp.PlainAuth("", e.From, e.Password, server)

	htmlBody := renderLoginHTMLTemplate(to, code)
	msg := []byte(
		"To: " + to + "\r\n" +
			"Subject: Your Login Code\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n" +
			htmlBody,
	)

	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%s", server, port), &tls.Config{
		InsecureSkipVerify: true, // Allow insecure connections (remove in production)
		ServerName:         server,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to the server: %v", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, server)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %v", err)
	}
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("failed to authenticate: %v", err)
	}
	if err := client.Mail(e.From); err != nil {
		return fmt.Errorf("failed to set sender: %v", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %v", err)
	}

	// Send the email body
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to send email data: %v", err)
	}
	_, err = writer.Write(msg)
	if err != nil {
		return fmt.Errorf("failed to write email: %v", err)
	}
	writer.Close()

	client.Quit()
	return nil
}

func main() {
	config.LoadConfig()
	sqlcliente, err := postgrescli.NewPostgresCli(
		config.Cfg.Postgres.User,
		config.Cfg.Postgres.Password,
		config.Cfg.Postgres.DBName,
		config.Cfg.Postgres.Host,
		config.Cfg.Postgres.Port,
		config.Cfg.Postgres.Ssl,
	)
	if err != nil {
		log.Fatalf("[%PostgreSQL] Error initializing POSTGRES client: %v\n", err)
	}
	app := &App{
		DB: sqlcliente, Throttle: make(map[string]time.Time),
		EmailSender: &EmailSender{
			From:     config.Cfg.Email.Email,
			Password: config.Cfg.Email.Password,
		},
	}

	r := mux.NewRouter()
	r.HandleFunc("/login/request", app.loginRequestHandler).Methods("POST")
	r.HandleFunc("/login/verify", app.loginVerifyHandler).Methods("POST")
	r.HandleFunc("/login/register", app.registerHandler).Methods("POST")

	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:5173", // For local dev (frontend)
			"https://play.retromindgames.pt",
			"https://play.retromindgames.pt",
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}).Handler(r)

	log.Println("Server running on :8081")
	http.ListenAndServe(":8081", corsHandler)
}

// Generate random 8-character code (uppercase letters and digits)
func generateLoginCode() (string, error) {
	randomBytes := make([]byte, 6)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return strings.ToUpper(base32.StdEncoding.EncodeToString(randomBytes)[:8]), nil
}

func (app *App) registerHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email        string `json:"email"`
		OperatorName string `json:"operator_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" {
		respondError(w, http.StatusBadRequest, "Email is required")
		return
	}

	_, err := app.DB.DB.Exec(`
		INSERT INTO users (Id, Email, OperatorName)
		VALUES (gen_random_uuid(), $1, $2)`,
		req.Email, req.OperatorName)

	if err != nil {
		log.Printf("Registration error: %v", err)
		respondJSON(w, http.StatusOK, Response{
			Success: true,
			Message: "User registered",
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "User registered",
	})
}

func (app *App) loginRequestHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	var responded bool
	defer func() {
		if !responded {
			respondJSON(w, http.StatusOK, Response{
				Success: true,
				Message: "Login code sent to email",
			})
		}
	}()

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request format")
		responded = true
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if last, ok := app.Throttle[req.Email]; ok && time.Since(last) < 60*time.Second {
		return
	}
	app.Throttle[req.Email] = time.Now()

	code, err := generateLoginCode()
	if err != nil {
		log.Printf("Code gen error: %v", err)
		return
	}

	var active bool
	err = app.DB.DB.QueryRow(`
		SELECT u.isactive
		FROM users u
		LEFT JOIN operators o ON u.operatorname = o.operatorname
		WHERE u.email = $1`, req.Email).Scan(&active)

	if err != nil || !active {
		log.Printf("Inactive or invalid user: %s", req.Email)
		return
	}

	expiresAt := time.Now().Add(15 * time.Minute)
	_, err = app.DB.DB.Exec(`
		UPDATE users 
		SET LoginCode = $1, CodeExpiresAt = $2, UpdatedAt = NOW() 
		WHERE Email = $3`, code, expiresAt, req.Email)

	if err != nil {
		log.Printf("DB update error for %s: %v", req.Email, err)
	}

	go app.sendEmail(req, code)
}

func (app *App) loginVerifyHandler(w http.ResponseWriter, r *http.Request) {
	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	var dbCode string
	var expiresAt time.Time
	err := app.DB.DB.QueryRow(`
		SELECT LoginCode, CodeExpiresAt 
		FROM users 
		WHERE Email = $1`,
		req.Email,
	).Scan(&dbCode, &expiresAt)

	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusUnauthorized, "Invalid credentials")
			return
		}
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Check code and expiration
	if dbCode != req.Code {
		respondError(w, http.StatusUnauthorized, "Invalid code")
		return
	}
	if time.Now().After(expiresAt) {
		respondError(w, http.StatusUnauthorized, "Code expired")
		return
	}
	_, err = app.DB.DB.Exec(`
		UPDATE users 
		SET LoginCode = NULL, CodeExpiresAt = NULL 
		WHERE email = $1`,
		req.Email)

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	tokenString, err := generateToken(req.Email, 24)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Token generation failed")
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Login successful",
		Token:   tokenString,
	})
}

func (app *App) sendEmail(req LoginRequest, code string) {
	if err := app.EmailSender.SendEmail(req.Email, code); err != nil {
		log.Printf("Failed to send email to %s: %v", req.Email, err)
	}
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, Response{
		Success: false,
		Message: message,
	})
}

func renderLoginHTMLTemplate(email, code string) string {
	result := htmlTemplate
	result = strings.ReplaceAll(result, "{email}", email)
	result = strings.ReplaceAll(result, "XXXXXX", code)
	return result
}
