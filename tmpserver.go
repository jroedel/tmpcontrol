package tmpcontrol

import (
	"context"
	_ "embed"
	"golang.org/x/time/rate"
	"log"
	"net"
	"sync"

	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"time"
)

type Server struct {
	l       Logger
	dbo     ServerDb
	Mux     *http.ServeMux
	Address string
}

//go:embed index.html
var indexHtmlContent string

func NewServer(dbFileName string, l Logger) (*Server, error) {
	db, err := NewSqliteServerDbFromFilename(dbFileName, l)
	if err != nil {
		return &Server{}, err
	}

	s := Server{dbo: db, l: l}

	mux := http.NewServeMux()

	mux.Handle("GET /", IndexCheck404Middleware(IndexCheckForFormGetSubmit(http.HandlerFunc(s.IndexHandler))))
	mux.HandleFunc("GET /configuration/{clientId}", s.GetConfigurationHandler)
	mux.HandleFunc("POST /configuration/{clientId}", s.PostConfigurationHandler)

	s.Mux = mux
	return &s, nil
}

func (s *Server) ListenAndServe() {
	s.l.Printf("server starting on %s", s.Address)
	if err := http.ListenAndServe(s.Address, WithLogger(s.l, LogRequestMiddleware(perClientRateLimiter(SecureHeadersMiddleware(s.Mux))))); err != nil {
		log.Fatalf("error: %s", err)
	}
}

func WithLogger(l Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l.Printf("Injecting l into request context")
		r = r.WithContext(context.WithValue(r.Context(), "l", l))
		next.ServeHTTP(w, r)
	})
}

// SecureHeadersMiddleware adds two basic security headers to each HTTP response
// X-XSS-Protection: 1; mode-block can help to prevent XSS attacks
// X-Frame-Options: deny can help to prevent clickjacking attacks
func SecureHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := r.Context().Value("l").(Logger)
		l.Printf("Req %s Setting the secure headers", r.Context().Value("requestId"))
		w.Header().Set("X-XSS-Protection", "1; mode-block")
		w.Header().Set("X-Frame-Options", "deny")
		next.ServeHTTP(w, r)
	})
}

func ValidateAndParseBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Executes middleware logic here...
		next.ServeHTTP(w, r)
	})
}

// LogRequestMiddleware logs basic info of a HTTP request
// RemoteAddr: Network address that sent the request (IP:port)
// Proto: Protocol version
// Method: HTTP method
// URL: Request URL
func LogRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := r.Context().Value("l").(Logger)
		requestId := randString(12)
		r = r.WithContext(context.WithValue(r.Context(), "requestId", requestId))
		start := time.Now()
		l.Printf("Req %s %s - %s %s %s\n", requestId, r.RemoteAddr, r.Proto, r.Method, r.URL)
		next.ServeHTTP(w, r)
		l.Printf("Req %s took %s\n", requestId, time.Since(start))
	})
}

func IndexCheck404Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			l := r.Context().Value("l").(Logger)
			l.Printf("We detected a 404: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func IndexCheckForFormGetSubmit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientId := r.FormValue("clientId")
		if clientId != "" {
			if ClientIdentifiersRegex.MatchString(clientId) {
				http.Redirect(w, r, "/configuration/"+clientId, http.StatusFound)
			} else {

			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) IndexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.New("index.html").Parse(indexHtmlContent)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = tmpl.Execute(w, nil)
	if err != nil {
		http.Error(w, "Internal server error", 500)
		return
	}
}

// Notification /*
type Notification struct {
	NotificationId      int       `json:"notificationId"`
	ReportedAt          time.Time `json:"reportedAt"`
	ClientId            string    `json:"clientId"`
	Message             string    `json:"message"`
	Severity            string    `json:"severity"`
	HasUserBeenNotified bool      `json:"hasUserBeenNotified"`
}

type ApiStatus int

const (
	NOK ApiStatus = iota + 1
	OK
)

func (s ApiStatus) String() string {
	switch s {
	case NOK:
		return "NOK"
	case OK:
		return "OK"
	}
	return ""
}

type ApiMessage struct {
	Status  ApiStatus `json:"status"`
	Message string    `json:"message"`
}

const maxAcceptedBodyLength = 1_000_000

var PostUnmarshalableJson error

func (s *Server) PostConfigurationHandler(w http.ResponseWriter, r *http.Request) {
	s.l.Printf("The server's PostConfigurationHandler just received a request: %s %s%s\n", r.Method, r.Host, r.RequestURI)
	w.Header().Set("Content-Type", "application/json")
	defer r.Body.Close()
	clientId := r.PathValue("clientId")
	if !ClientIdentifiersRegex.MatchString(clientId) {
		//not a valid clientId
		dispatchApiError(w, http.StatusBadRequest, "Invalid clientId", s.l)
		return
	}
	fmt.Printf("clientId: %#v\n", clientId)

	// Step 1: Get, convert & validate the data
	rdr := io.LimitReader(r.Body, maxAcceptedBodyLength)
	data, err := io.ReadAll(rdr)
	if err != nil {
		s.l.Printf("error: can't read - %s", err)
		dispatchApiError(w, http.StatusBadRequest, "can't read body", s.l)
		return
	}
	if len(data) == maxAcceptedBodyLength {
		s.l.Printf("they sent us too much data")
		dispatchApiError(w, http.StatusBadRequest, "body too long", s.l)
		return
	}

	if len(data) == 0 {
		dispatchApiError(w, http.StatusBadRequest, "missing body", s.l)
		return
	}
	var config ControllersConfig
	err = json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		s.PostJsonConfigurationHandler(w, r, s.l, PostResult{config: ControllersConfig{}, err: PostUnmarshalableJson})
		return
	}
	fmt.Printf("config: %#v\n", config)
	err = s.dbo.CreateOrUpdateConfig(clientId, config)
	if err != nil {
		s.PostJsonConfigurationHandler(w, r, s.l, PostResult{config: config, err: err})
		return
	}

	r.WithContext(context.WithValue(r.Context(), "controllerConfig", config))
	s.PostJsonConfigurationHandler(w, r, s.l, PostResult{config: config})
}

// write the status code and the json error message
func dispatchApiError(w http.ResponseWriter, httpStatus int, message string, l Logger) {
	w.WriteHeader(httpStatus)
	err := json.NewEncoder(w).Encode(ApiMessage{Status: NOK, Message: message})
	if err != nil {
		l.Printf("Error encoding json: %v", err)
	}
}

type PostResult struct {
	config ControllersConfig
	err    error
}

func (s *Server) PostJsonConfigurationHandler(w http.ResponseWriter, r *http.Request, l Logger, result PostResult) {
	if result.err != nil {
		if errors.Is(result.err, PostUnmarshalableJson) {
			dispatchApiError(w, http.StatusBadRequest, "invalid request", l)
			return
		}
		dispatchApiError(w, http.StatusInternalServerError, "issue writing to database", l)
		return
	}
	w.WriteHeader(http.StatusOK)
	s2 := `{"result": "OK"}`
	_, _ = w.Write([]byte(s2))
}

func (s *Server) PostHtmlConfigurationHandler(w http.ResponseWriter, r *http.Request, result PostResult) {

}

func (s *Server) GetConfigurationHandler(w http.ResponseWriter, r *http.Request) {
	s.l.Printf("The server's GetConfigurationHandler just received a request: %s %s%s\n", r.Method, r.Host, r.RequestURI)
	w.Header().Set("Content-Type", "application/json")
	defer r.Body.Close()
	clientId := r.PathValue("clientId")
	if !ClientIdentifiersRegex.MatchString(clientId) {
		//not a valid clientId
		w.WriteHeader(http.StatusBadRequest)
		s2 := `{"result": "NOK","message":"invalid client id"}`
		_, _ = w.Write([]byte(s2))
		// let's just ignore the error since we've done all we can
		return
	}
	fmt.Printf("clientId: %#v\n", clientId)
	//var result []byte

	config, ok, err := s.dbo.GetConfig(clientId)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s2 := `{"result": "NOK","message":"internal server error"}`
		_, _ = w.Write([]byte(s2))
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		s2 := `{"result": "NOK","message":"not found"}`
		_, _ = w.Write([]byte(s2))
		return
	}
	configBytes, err := json.Marshal(config)

	//w.WriteHeader(http.StatusOK)
	_, err = w.Write(configBytes)
	if err != nil {
		s.l.Printf("There was an issue writing the content to the client: %s", err.Error())
	}
}

func IsJSON(str []byte) bool {
	var js json.RawMessage
	return json.Unmarshal(str, &js) == nil
}

func perClientRateLimiter(next http.Handler) http.Handler {
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}
	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)
	go func() {
		for {
			time.Sleep(time.Minute)
			// Lock the mutex to protect this section from race conditions.
			mu.Lock()
			for ip, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract the IP address from the request.
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Lock the mutex to protect this section from race conditions.
		mu.Lock()
		if _, found := clients[ip]; !found {
			clients[ip] = &client{limiter: rate.NewLimiter(2, 4)}
		}
		clients[ip].lastSeen = time.Now()
		if !clients[ip].limiter.Allow() {
			mu.Unlock()

			message := ApiMessage{
				Status:  NOK,
				Message: "The API is at capacity, try again later.",
			}

			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(&message)
			return
		}
		mu.Unlock()
		next.ServeHTTP(w, r)
	})
}
