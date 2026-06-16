package main

import (
	"context"
	"embed"
	"encoding/base64"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/bytedance/gopkg/util/logger"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/option"
)

// Embed static files at compile time
//
//go:embed public/*
var staticFiles embed.FS

type item struct {
	ID     string  `json:"id"`
    Name   string  `json:"name"`
    Description  string  `json:"description"`
    Quantity float64  `json:"quantity"`
    Price  float64 `json:"price"`
}

// seed item data.
var items = []item{
    {ID: "1", Name: "Banana", Description: "Fruit", Quantity: 1, Price: 2},
    {ID: "2", Name: "Cheesecake", Description: "Pastry", Quantity: 4, Price: 8},
    {ID: "3", Name: "Salmon", Description: "Fish", Quantity: 3, Price: 16},
}

type Handler struct {
	AuthClient *auth.Client
}

func main() {
	// 1. Define background context
	ctx := context.Background()

	// 2. Load the Firebase service account credential file
	decodedBytes, err := base64.StdEncoding.DecodeString(os.Getenv("FIREBASE_KEY"))
		if err != nil {
			log.Fatalf("Decoding failed: %v", err)
		}

	opt := option.WithCredentialsJSON([]byte(decodedBytes))

	// 3. Initialize the Firebase App
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		log.Fatalf("error initializing firebase app: %v\n", err)
	}

	// 4. Connect to the Firebase Auth client instance
	authClient, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("error getting firebase auth client: %v\n", err)
	}

	log.Println("Successfully connected to Firebase Auth instance!", authClient)

	handler := &Handler{AuthClient: authClient}

	// feUrl := os.Getenv("FE_URL")
	// gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
 	config := cors.DefaultConfig()
    config.AllowOrigins = []string{"https://ims-ui-two.vercel.app"}
    config.AllowMethods = []string{"POST", "GET", "PUT", "OPTIONS", "DELETE"}
    config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization", "Accept", "User-Agent", "Cache-Control", "Pragma"}
    config.ExposeHeaders = []string{"Content-Length"}
    config.AllowCredentials = true
    config.MaxAge = 12 * time.Hour

    // Use CORS middleware
    r.Use(cors.New(config))

	// Serve embedded static files
	// Strip "public" prefix so files are served at root (e.g., /index.html, /favicon.ico)
	publicFS, _ := fs.Sub(staticFiles, "public")
	r.StaticFS("/static", http.FS(publicFS))

	// Serve index.html at root
	r.GET("/", func(c *gin.Context) {
		data, err := staticFiles.ReadFile("public/index.html")
		if err != nil {
			c.String(http.StatusNotFound, "Not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	// Serve favicon at root
	r.GET("/favicon.ico", func(c *gin.Context) {
		data, err := staticFiles.ReadFile("public/favicon.ico")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "image/x-icon", data)
	})

	// API routes
	r.GET("/items", handler.getItems)
	r.GET("/items/:id", handler.getItemByID)
	r.POST("/items", handler.postItems)
	r.PUT("/items/:id", handler.updateItemByID)
	r.DELETE("/items/:id", handler.deleteItemByID)

	// Get port from environment variable (Vercel sets this)
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	r.Run(":" + port)
}

// authorize by checking firebase auth
func authFirebase(c *gin.Context, firebaseAuth *auth.Client) bool{
	token := c.GetHeader("Authorization")
	if token == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return false
	}
	_, err := firebaseAuth.VerifyIDToken(c, token)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return false
	}
	return true
}

func (h *Handler) getItems(c *gin.Context) {
	if !authFirebase(c, h.AuthClient) {
		return
	}
    c.IndentedJSON(http.StatusOK, items)
}

func (h *Handler) postItems(c *gin.Context) {
	if !authFirebase(c, h.AuthClient) {
		return
	}
    var newItem item

    if err := c.BindJSON(&newItem); err != nil {
    	// log error
    	logger.Error(err)
        return
    }

    // check if item ID already exists
    for _, a := range items {
        if a.ID == newItem.ID {
            c.IndentedJSON(http.StatusConflict, gin.H{"message": "item id already exists"})
            return
        }
    }

    // get the latest available id by finding the highest existing id
    latestID := 0
    for _, a := range items {
        id, _ := strconv.Atoi(a.ID)
        if id > latestID {
            latestID = id
        }
    }
    newItem.ID = strconv.Itoa(latestID + 1)

    // check if item name already exists
    for _, a := range items {
        if a.Name == newItem.Name {
            c.IndentedJSON(http.StatusConflict, gin.H{"message": "item name already exists"})
            return
        }
    }

    // Add the new album to the slice.
    items = append(items, newItem)
    c.IndentedJSON(http.StatusCreated, newItem)
}

func (h *Handler) getItemByID(c *gin.Context) {
	if !authFirebase(c, h.AuthClient) {
		return
	}
    id := c.Param("id")

    // Loop over the list of albums, looking for
    // an album whose ID value matches the parameter.
    for _, a := range items {
        if a.ID == id {
            c.IndentedJSON(http.StatusOK, a)
            return
        }
    }
    c.IndentedJSON(http.StatusNotFound, gin.H{"message": "item not found"})
}

func (h *Handler) updateItemByID(c *gin.Context) {
	if !authFirebase(c, h.AuthClient) {
		return
	}
    id := c.Param("id")
    var updatedItem item

    if err := c.BindJSON(&updatedItem); err != nil {
        return
    }

    for i, a := range items {
        if a.ID == id {
            items[i] = updatedItem
            c.IndentedJSON(http.StatusOK, updatedItem)
            return
        }
    }
    c.IndentedJSON(http.StatusNotFound, gin.H{"message": "item not found"})
}

func (h *Handler) deleteItemByID(c *gin.Context) {
	if !authFirebase(c, h.AuthClient) {
		return
	}
    id := c.Param("id")

    for i, a := range items {
        if a.ID == id {
            items = append(items[:i], items[i+1:]...)
            c.IndentedJSON(http.StatusOK, gin.H{"message": "item deleted"})
            return
        }
    }
    c.IndentedJSON(http.StatusNotFound, gin.H{"message": "item not found"})
}
