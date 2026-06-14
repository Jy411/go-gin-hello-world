package main

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bytedance/gopkg/util/logger"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
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

func main() {
	// feUrl := os.Getenv("FE_URL")
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
 	config := cors.DefaultConfig()
    config.AllowOrigins = []string{"*"}
    config.AllowMethods = []string{"POST", "GET", "PUT", "OPTIONS", "DELETE"}
    config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization", "Accept", "User-Agent", "Cache-Control", "Pragma"}
    config.ExposeHeaders = []string{"Content-Length"}
    config.AllowCredentials = true
    config.MaxAge = 12 * time.Hour


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
	r.GET("/items", getItems)
	r.GET("/items/:id", getItemByID)
	r.POST("/items", postItems)
	r.PUT("/items/:id", updateItemByID)
	r.DELETE("/items/:id", deleteItemByID)

	// Get port from environment variable (Vercel sets this)
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	r.Run(":" + port)
}


func getItems(c *gin.Context) {
    c.IndentedJSON(http.StatusOK, items)
}

func postItems(c *gin.Context) {
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

    // modify id to the next available id
    newItem.ID = strconv.Itoa(len(items) + 1)

    // Add the new album to the slice.
    items = append(items, newItem)
    c.IndentedJSON(http.StatusCreated, newItem)
}

func getItemByID(c *gin.Context) {
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

func updateItemByID(c *gin.Context) {
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

func deleteItemByID(c *gin.Context) {
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
