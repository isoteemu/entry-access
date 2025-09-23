package main

// HTTP GIN handler for generating QR codes

// func main() {
// 	r := gin.Default()

// 	r.Static("/static", "./assets") // Serve CSS
// 	r.LoadHTMLGlob("templates/*")

// 	r.GET("/", func(c *gin.Context) {
// 		c.HTML(http.StatusOK, "index.tpl.html", gin.H{})
// 	})

// 	r.GET("/ping", func(c *gin.Context) {
// 		c.JSON(http.StatusOK, gin.H{
// 			"message": "pong",
// 		})
// 	})

// 	r.GET("/provision", func(c *gin.Context) {
// 		// TODO: Implement provisioning interface
// 	})

// 	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
// }
