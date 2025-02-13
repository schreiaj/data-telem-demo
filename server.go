package main

import (
	"database/sql"
	"fmt"
	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/schreiaj/data-telem/views"
	datastar "github.com/starfederation/datastar/sdk/go"
	_ "github.com/tursodatabase/go-libsql"
	"math/rand/v2"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

func main() {
	//dir, err := os.MkdirTemp("", "libsql-*")
	//
	//if err != nil {
	//	panic(err)
	//}
	dir := "."
	defer os.RemoveAll(dir)
	fmt.Println(dir)
	db, err := sql.Open("libsql", "file:"+dir+"/test.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	fields := []string{
		"QPS",
		"Latency",
		"Error Rate",
		"CPU Usage",
		"Memory Usage",
		"Request Throughput",
		"Response Time",
		"Disk IO",
		"Network Traffic",
		"Active Connections",
		"Service Health Status",
		"Throughput per Service",
	}
	for i, field := range fields {
		fields[i] = strings.ReplaceAll(strings.ToLower(field), " ", "_")
	}

	db.Exec("PRAGMA journal_mode = WAL")
	db.Exec("PRAGMA busy_timeout = 5000")
	db.Exec("PRAGMA synchronous = NORMAL")
	db.Exec("PRAGMA cache_size = 1000000000")
	db.Exec("PRAGMA foreign_keys = true")
	db.Exec("PRAGMA temp_store = memory")
	db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY AUTOINCREMENT, key TEXT, data REAL)")
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		templ.Handler(views.Index(fields)).ServeHTTP(w, r)
	})

	r.Get("/stream", func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-r.Context().Done():
					return
				default:

					for _, field := range fields {
						var vals []float64
						rows, err := db.Query("SELECT data FROM test WHERE key = ? ORDER BY id DESC LIMIT 100", field)
						if err != nil {
							panic(err)
						}
						for rows.Next() {
							var v float64
							rows.Scan(&v)
							v = v * 150
							vals = append(vals, v)
						}

						slices.Reverse(vals)
						svg, _ := generateSVGPath(vals, 1000, 150)
						sse.MergeSignals([]byte(fmt.Sprintf("{%s: '%s'}", field, svg)))
					}
				}
				time.Sleep(16 * time.Millisecond)
			}
		}()
		wg.Wait()

	})

	//This is a function that just generates random data and inserts it into the database
	go func() {
		for {
			time.Sleep(50 * time.Millisecond)
			for _, field := range fields {
				val := rand.Float64()
				_, err := db.Exec("INSERT INTO test (key, data) VALUES (?, ?)", field, val)
				if err != nil {
				}
			}
		}
	}()
	http.ListenAndServe(":3000", r)
}

// generateSVGPath generates the path string for an SVG line chart from an array of floats.
func generateSVGPath(data []float64, width, height float64) (string, error) {
	// If the data is empty, return an error.
	if len(data) == 0 {
		return "", fmt.Errorf("data array cannot be empty")
	}

	// Start the SVG path string with the "M" command to move to the first point.
	var path strings.Builder
	path.WriteString(fmt.Sprintf("M %.2f %.2f ", 0.0, height-data[0])) // Initial point (x, y)

	// Iterate through the data and generate the line path.
	for i := 1; i < len(data); i++ {
		// Each point (x, y) where x is the index and y is the data value.
		x := float64(i) * (width / float64(len(data)-1))    // scale x according to chart width
		y := height - data[i]                               // invert y to fit the SVG coordinate system
		path.WriteString(fmt.Sprintf("L %.2f %.2f ", x, y)) // Add a line to the next point
	}

	return path.String(), nil
}
