package main

import (
  "database/sql"
  "errors"
  "fmt"
  "log"
  "net/http"
  "os"
  "strconv" 

  "github.com/gin-gonic/gin"
  "github.com/go-sql-driver/mysql"
)

var db *sql.DB

type Album struct {
  ID     int64   `json:"id"`
  Title  string  `json:"title"`
  Artist string  `json:"artist"`
  Price  float32 `json:"price"`
}

func main() {
  cfg := mysql.NewConfig()
  cfg.User = os.Getenv("DBUSER")
  cfg.Passwd = os.Getenv("DBPASS")
  cfg.Net = "tcp"
  cfg.Addr = "127.0.0.1:3306"
  cfg.DBName = "recordings"

  var err error
  db, err = sql.Open("mysql", cfg.FormatDSN())
  if err != nil {
    log.Fatal(err)
  }

  pingErr := db.Ping()
  if pingErr != nil {
    log.Fatal(pingErr)
  }
  fmt.Println("Connected to the database!")

  // Set up the Gin router and define all endpoints.
  router := gin.Default()
  router.GET("/albums/:artist", getAlbumsByArtist)
  router.GET("/album/:id", getAlbumByID)
  router.POST("/albums", postAlbum)      

  // Start the server.
  router.Run("localhost:8080")
}

// postAlbum adds an album from JSON received in the request body.
func postAlbum(c *gin.Context) {
  var newAlbum Album

  // Bind the received JSON to the newAlbum struct.
  if err := c.BindJSON(&newAlbum); err != nil {
    c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "invalid JSON format"})
    return
  }

  // Call the helper function to add the album to the database.
  id, err := addAlbum(newAlbum)
  if err != nil {
    log.Printf("Error adding album: %v", err)
    c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "could not add album"})
    return
  }

  // Set the ID of the newly created album and return it.
  newAlbum.ID = id
  c.IndentedJSON(http.StatusCreated, newAlbum)
}

// getAlbumByID locates an album by its ID from a URL parameter.
func getAlbumByID(c *gin.Context) {
  // Get the ID from the URL parameter.
  idStr := c.Param("id")
  id, err := strconv.ParseInt(idStr, 10, 64)
  if err != nil {
    c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "invalid album ID"})
    return
  }

  // Call the helper function to get the album from the database.
  album, err := albumByID(id)
  if err != nil {
    // Check if the error is a "not found" error.
    if errors.Is(err, sql.ErrNoRows) {
      c.IndentedJSON(http.StatusNotFound, gin.H{"message": "album not found"})
      return
    }
    log.Printf("Error getting album by ID: %v", err)
    c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "database error"})
    return
  }

  c.IndentedJSON(http.StatusOK, album)
}

func getAlbumsByArtist(c *gin.Context) {
  artist := c.Param("artist")
  var albums []Album

  rows, err := db.Query("SELECT * FROM album WHERE artist ='" + artist + "'")
  if err != nil {
    log.Printf("Error querying database: %v", err)
    c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "database query error"})
    return
  }
  defer rows.Close()

  for rows.Next() {
    var alb Album
    if err := rows.Scan(&alb.ID, &alb.Title, &alb.Artist, &alb.Price); err != nil {
      log.Printf("Error scanning row: %v", err)
      c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "error processing results"})
      return
    }
    albums = append(albums, alb)
  }

  if err = rows.Err(); err != nil {
    log.Printf("Error iterating rows: %v", err)
    c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "error processing results"})
    return
  }
  
  if len(albums) == 0 {
    c.IndentedJSON(http.StatusNotFound, gin.H{"message": "no albums found for artist"})
    return
  }

  c.IndentedJSON(http.StatusOK, albums)
}


func albumByID(id int64) (Album, error) {
  var alb Album

  row := db.QueryRow("SELECT * FROM album WHERE id = ?", id)
  if err := row.Scan(&alb.ID, &alb.Title, &alb.Artist, &alb.Price); err != nil {
    // Return the original sql.ErrNoRows error.
    if err == sql.ErrNoRows {
      return alb, sql.ErrNoRows
    }
    return alb, fmt.Errorf("albumByID %d: %v", id, err)
  }
  return alb, nil
}


func addAlbum(alb Album) (int64, error) {
  result, err := db.Exec("INSERT INTO album (title, artist, price) VALUES (?, ?, ?)", alb.Title, alb.Artist, alb.Price)
  if err != nil {
    return 0, fmt.Errorf("addAlbum: %v", err)
  }
  id, err := result.LastInsertId()
  if err != nil {
    return 0, fmt.Errorf("addAlbum: %v", err)
  }
  return id, nil
}