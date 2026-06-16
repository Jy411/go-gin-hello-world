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
	"github.com/jackc/pgx/v5"
	"google.golang.org/api/option"
)

// Embed static files at compile time
//
//go:embed public/*
var staticFiles embed.FS

type item struct {
	ID     string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Quantity    float64 `json:"quantity"`
}

// seed item data.
var items = []item{
}

type Handler struct {
	AuthClient *auth.Client
	DbClient   *pgx.Conn
}

func main() {
	ctx := context.Background()

	// Connect to DB - supabase
	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	defer conn.Close(ctx)

	decodedBytes, err := base64.StdEncoding.DecodeString(os.Getenv("FIREBASE_KEY"))
		if err != nil {
			log.Fatalf("Decoding failed: %v", err)
		}
	opt := option.WithCredentialsJSON([]byte(decodedBytes))
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		log.Fatalf("error initializing firebase app: %v\n", err)
	}
	authClient, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("error getting firebase auth client: %v\n", err)
	}

	log.Println("Successfully connected to Firebase Auth instance!", authClient)

	handler := &Handler{AuthClient: authClient, DbClient: conn}

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

	// query the database for items
	rows, err := h.DbClient.Query(c, "SELECT * FROM items")
	if err != nil {
		logger.Error(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var items []item
	for rows.Next() {
		var item item
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Price, &item.Quantity); err != nil {
			logger.Error(err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		logger.Error(err)
		c.AbortWithStatus(http.StatusInternalServerError)
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

    // query database for latest id and assign id
    var latestID int
    if err := h.DbClient.QueryRow(c, "SELECT MAX(id) FROM items").Scan(&latestID); err != nil {
        logger.Error(err)
        return
    }
    newItem.ID = strconv.Itoa(latestID + 1)

    // insert new item into database
    if _, err := h.DbClient.Exec(c, "INSERT INTO items (id, name, description, price, quantity) VALUES ($1, $2, $3, $4, $5)", newItem.ID, newItem.Name, newItem.Description, newItem.Price, newItem.Quantity); err != nil {
    	c.IndentedJSON(http.StatusConflict, gin.H{"message": err.Error()})
        return
    }

    c.IndentedJSON(http.StatusCreated, newItem)
}

func (h *Handler) getItemByID(c *gin.Context) {
	if !authFirebase(c, h.AuthClient) {
		return
	}
    id := c.Param("id")

    var item item
    if err := h.DbClient.QueryRow(c, "SELECT * FROM items WHERE id = $1", id).Scan(&item.ID, &item.Name, &item.Description, &item.Price, &item.Quantity); err != nil {
    	log.Println(err);
        c.IndentedJSON(http.StatusNotFound, gin.H{"message": "item not found"})
        return
    }

    c.IndentedJSON(http.StatusOK, item)
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

    if _, err := h.DbClient.Exec(c, "UPDATE items SET name = $1, description = $2, price = $3, quantity = $4 WHERE id = $5", updatedItem.Name, updatedItem.Description, updatedItem.Price, updatedItem.Quantity, id); err != nil {
        c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
        return
    }

    c.IndentedJSON(http.StatusOK, updatedItem)
}

func (h *Handler) deleteItemByID(c *gin.Context) {
	if !authFirebase(c, h.AuthClient) {
		return
	}
    id := c.Param("id")


    if _, err := h.DbClient.Exec(c, "DELETE FROM items WHERE id = $1", id); err != nil {
        c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
        return
    }

    c.IndentedJSON(http.StatusOK, gin.H{"message": "item deleted"})
}
