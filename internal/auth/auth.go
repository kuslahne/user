package auth

import (
	"crypto/sha1"
	"fmt"
	"io"
	"strconv"
	"net/http"
	"time"
	"strings"

	"github.com/globalsign/mgo/bson"
	"github.com/gin-gonic/gin"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/vmwarecloudadvocacy/user/pkg/logger"
	
)

var (
	ErrMissingField         = "Error missing %v"
	// AtJwtKey is used to create the Access token signature
	AtJwtKey = []byte("my_secret_key")
	// RtJwtKey is used to create the refresh token signature
	RtJwtKey = []byte("my_secret_key_2")
	
)

type TokenRequestBody struct {
	RefreshToken string `json:"refresh_token"`
}

type registerRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Email     string `json:"email"`
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
}

type UserResponse struct {
	Username  string        `json:"username"`
	Email     string        `json:"email"`
	FirstName string        `json:"firstname"`
	LastName  string        `json:"lastname"`
	ID        bson.ObjectId `json:"id" bson:"_id"`
}

// User Struct
type User struct {
	FirstName string        `json:"firstname" bson:"firstname"`
	LastName  string        `json:"lastname" bson:"lastname"`
	Email     string        `json:"email" bson:"email"`
	Username  string        `json:"username" bson:"username"`
	Password  string        `json:"password" bson:"password"`
	ID        bson.ObjectId `json:"id" bson:"_id"`
	Salt      string        `json:"-" bson:"salt"`
}

// Validate if the fields are available
func (u *User) Validate() error {
	if u.FirstName == "" {
		return fmt.Errorf(ErrMissingField, "FirstName")
	}
	if u.LastName == "" {
		return fmt.Errorf(ErrMissingField, "LastName")
	}
	if u.Username == "" {
		return fmt.Errorf(ErrMissingField, "Username")
	}
	if u.Password == "" {
		return fmt.Errorf(ErrMissingField, "Password")
	}
	return nil
}

func (u *User) NewSalt() {
	h := sha1.New()
	io.WriteString(h, strconv.Itoa(int(time.Now().UnixNano())))
	u.Salt = fmt.Sprintf("%x", h.Sum(nil))
}

func CalculatePassHash(pass, salt string) string {
	h := sha1.New()
	io.WriteString(h, salt)
	io.WriteString(h, pass)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// AuthMiddleware checks if the JWT sent is valid or not. This function is involked for every API route that needs authentication
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientToken := c.GetHeader("Authorization")
		if clientToken == "" {
			logger.Logger.Errorf("Authorization token was not provided")
			c.JSON(http.StatusUnauthorized, gin.H{"status": http.StatusUnauthorized, "message": "Authorization Token is required"})
			c.Abort()
			return
		}

		claims := jwt.MapClaims{}

		extractedToken := strings.Split(clientToken, "Bearer ")

		// Verify if the format of the token is correct
		if len(extractedToken) == 2 {
			clientToken = strings.TrimSpace(extractedToken[1])
		} else {
			logger.Logger.Errorf("Incorrect Format of Authn Token")
			c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Incorrect Format of Authorization Token "})
			c.Abort()
			return
		}

		// Parse the claims
		parsedToken, err := jwt.ParseWithClaims(clientToken, claims, func(token *jwt.Token) (interface{}, error) {
			return AtJwtKey, nil
		})

		if err != nil {
			if err == jwt.ErrSignatureInvalid {
				logger.Logger.Errorf("Invalid Token Signature")
				c.JSON(http.StatusUnauthorized, gin.H{"status": http.StatusUnauthorized, "message": "Invalid Token"})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Bad Request"})
			c.Abort()
			return
		}

		if !parsedToken.Valid {
			logger.Logger.Errorf("Invald Token")
			c.JSON(http.StatusUnauthorized, gin.H{"status": http.StatusUnauthorized, "message": "Invalid Token"})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

func GenerateToken(username string, uuid string) (string, string, error) {

	tokenString, err := GenerateAccessToken(username, uuid)
	if err !=nil {
		return "", "", err
	}

	// Create Refresh token, this will be used to get new access token.
	refreshToken := jwt.New(jwt.SigningMethodHS256)

	expirationTimeRefreshToken := time.Now().Add(15 * time.Minute).Unix()

	rtClaims := refreshToken.Claims.(jwt.MapClaims)
	rtClaims["sub"] = uuid
	rtClaims["exp"] = expirationTimeRefreshToken

	refreshTokenString, err := refreshToken.SignedString(RtJwtKey)
	if err != nil {
		return "", "", err
	}

	return  tokenString, refreshTokenString, nil
}

func ValidateRefreshToken(reqRefreshToken string) (bool, string, error) {

	claims := jwt.MapClaims{}

	refreshToken, err := jwt.ParseWithClaims(reqRefreshToken, claims, func(token *jwt.Token) (interface{},error) {
		return RtJwtKey, nil
	})

	if err != nil {
		if err == jwt.ErrSignatureInvalid {
			logger.Logger.Errorf("Invalid Token Signature")
			return false, "", err
		}
		return false, "", err
	}

	if !refreshToken.Valid {
		logger.Logger.Errorf("Invalid Token")
		return false, "", err
	}

	return true, claims["sub"].(string), nil
}

func GenerateAccessToken(username string, uuid string) (string, error) {
	// Declare the expiration time of the access token
	// Here the expiration is 5 minutes
	expirationTimeAccessToken := time.Now().Add(5 * time.Minute).Unix()

	// Declare the token with the algorithm used for signing, and the claims
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["Username"] = username
	claims["exp"] = expirationTimeAccessToken
	claims["sub"] = uuid

	// Create the JWT string
	tokenString, err := token.SignedString(AtJwtKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}