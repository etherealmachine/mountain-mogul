package sim

import (
	"fmt"
	"net/http"

	"mountain-mogul/internal/world"
)

// QueryServer exposes a live SQL query endpoint for the running sim.
// Queries are dispatched to the main game thread via a channel so there
// is no concurrent access to sim or world state.
//
// Start it with NewQueryServer(addr).Start(), then call Tick every frame
// from the game loop. Query with:
//
//	curl -G http://localhost:6061/query --data-urlencode "sql=SELECT x,z,avy_snow FROM cells WHERE avy_snow > 0"
//	curl http://localhost:6061/schema
type QueryServer struct {
	addr     string
	requests chan queryReq
}

type queryReq struct {
	sql    string
	result chan string
}

// NewQueryServer creates a QueryServer that will listen on addr (e.g. "localhost:6061").
func NewQueryServer(addr string) *QueryServer {
	return &QueryServer{
		addr:     addr,
		requests: make(chan queryReq, 16),
	}
}

// Start launches the HTTP listener in a background goroutine.
func (qs *QueryServer) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/query", qs.handleQuery)
	mux.HandleFunc("/schema", qs.handleSchema)
	go func() {
		fmt.Printf("query server: http://%s/query  (GET ?sql=…)\n", qs.addr)
		if err := http.ListenAndServe(qs.addr, mux); err != nil {
			fmt.Println("query server:", err)
		}
	}()
}

// Tick drains any pending queries on the caller's goroutine (must be the
// game/main thread). Builds a TableSet only when a query is waiting.
func (qs *QueryServer) Tick(w *world.World, s *Simulation) {
	for {
		select {
		case req := <-qs.requests:
			ts := BuildTableSet(w, s)
			result, err := RunQuery(ts, req.sql)
			if err != nil {
				req.result <- "ERROR: " + err.Error() + "\n"
			} else {
				req.result <- result
			}
		default:
			return
		}
	}
}

func (qs *QueryServer) handleQuery(w http.ResponseWriter, r *http.Request) {
	sql := r.URL.Query().Get("sql")
	if sql == "" {
		http.Error(w, "missing ?sql= parameter", http.StatusBadRequest)
		return
	}
	result := make(chan string, 1)
	qs.requests <- queryReq{sql: sql, result: result}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, <-result)
}

// handleSchema returns the column names for each table, derived from a
// zero-value world so the schema is available without a live sim.
func (qs *QueryServer) handleSchema(w http.ResponseWriter, r *http.Request) {
	result := make(chan string, 1)
	qs.requests <- queryReq{sql: "SELECT * FROM cells LIMIT 0; SELECT * FROM guests LIMIT 0; SELECT * FROM cats LIMIT 0; SELECT * FROM buildings LIMIT 0; SELECT * FROM lifts LIMIT 0; SELECT * FROM patrollers LIMIT 0", result: result}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, <-result)
}
