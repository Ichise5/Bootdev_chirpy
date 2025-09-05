package main

import (
	"os"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/Ichise5/Chirpy/internal/database"
	"database/sql"
)

func main(){
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	dbQueries := database.New(db)

	var apiCfg apiConfig

	const readinessPath = "/api/healthz"
	const filepathRoot = "app"
	const metricsPath = "/api/metrics"
	const resetPath = "/api/reset"
	const htmlMetricsPath = "/admin/metrics"
	const htmlResetPath = "/admin/reset"
	const validatePath = "/api/validate_chirp"
	const port = "8080"

	mux := http.NewServeMux()

	mux.Handle("/app", apiCfg.middlewareMetricsInc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepathRoot+"/index.html")
	})))

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(filepathRoot)))))

	mux.HandleFunc(readinessPath, readinessHandler)
	mux.HandleFunc(metricsPath, apiCfg.AddHitHandler)
	mux.HandleFunc(htmlMetricsPath, apiCfg.AddAdminHitHandler)
	mux.HandleFunc(resetPath, apiCfg.ResetHitHandler)
	mux.HandleFunc(htmlResetPath, apiCfg.AdminResetHandler)
	mux.HandleFunc(validatePath, apiCfg.ValidateChirp)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Serving files from %s on port: %s\n", filepathRoot, port)
	log.Fatal(srv.ListenAndServe())

}

type InputStruct struct{
	Body string `json:"body"`	
}
type ErrorStrucr struct{
	Error string `json:"error"`
}
type OutputStruct struct {
	Cleaned_body string `json:"cleaned_body"`
}

func readinessHandler(w http.ResponseWriter, r *http.Request){
	if r.Method != http.MethodGet {
        w.WriteHeader(http.StatusMethodNotAllowed)
        return
    }
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

type apiConfig struct {
	fileserverHits atomic.Int32

}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w,r)
		})
	}

func (cfg *apiConfig) AddHitHandler(w http.ResponseWriter, r *http.Request){
	if r.Method != http.MethodGet {
        w.WriteHeader(http.StatusMethodNotAllowed)
        return
    }
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte(fmt.Sprintf("Hits: %v", cfg.fileserverHits.Load())))
}



func (cfg *apiConfig) ResetHitHandler(w http.ResponseWriter, r *http.Request){
	if r.Method != http.MethodPost {
        w.WriteHeader(http.StatusMethodNotAllowed)
        return
    }
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte(fmt.Sprintf("Hits: %v", cfg.fileserverHits.Load())))
}


func (cfg *apiConfig) AddAdminHitHandler(w http.ResponseWriter, r *http.Request){
	if r.Method != http.MethodGet {
        w.WriteHeader(http.StatusMethodNotAllowed)
        return
    }
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte(chirpy_visit(cfg.fileserverHits.Load())))
}

func (cfg *apiConfig) AdminResetHandler(w http.ResponseWriter, r *http.Request){
	if r.Method != http.MethodPost {
        w.WriteHeader(http.StatusMethodNotAllowed)
        return
    }
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte(chirpy_visit(cfg.fileserverHits.Load())))
}


func chirpy_visit(hits int32) string{
	return fmt.Sprintf(`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, hits)
}

func (cfg *apiConfig) ValidateChirp(w http.ResponseWriter, r *http.Request){
	if r.Method != http.MethodPost {
        w.WriteHeader(http.StatusMethodNotAllowed)
        return
    }

	var body InputStruct
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&body)

	if err != nil{
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
        return
    }

	if len(body.Body)>140{
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
        return
	}else{
		checkedChirp := checkChirp(body.Body)
		respBody := OutputStruct{
			Cleaned_body: checkedChirp,
		}
		respondWithJSON(w, http.StatusOK, respBody)
	}
}

func respondWithError(w http.ResponseWriter, code int, msg string){
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(code)
		w.Write(fmt.Appendf(nil, msg))
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}){
		data, err := json.Marshal(payload)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Error marshalling JSON: %s", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		w.Write(data)
}


func checkChirp(chirp string) string{
	newChirpArr := strings.Split(chirp," ")
	illegalWords := [3]string{"kerfuffle", "sharbert", "fornax"}

	for idx,word := range(newChirpArr){
		for _,illWord := range(illegalWords){
			if strings.ToLower(word) == illWord{
				newChirpArr[idx] = "****"
			}
		}
	}
	return strings.Join(newChirpArr," ")
}