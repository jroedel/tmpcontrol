package tmpcontrol

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Server struct {
	logger Logger
	dbo    ServerDb
}

func NewServer(dbFileName string, logger Logger) (*Server, error) {
	db, err := NewSqliteServerDbFromFilename(dbFileName, logger)
	if err != nil {
		return &Server{}, err
	}
	return &Server{dbo: db, logger: logger}, nil
}

func (s *Server) GetConfigurationHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Printf("The server's GetConfigurationHandler just received a request: %s %s%s\n", r.Method, r.Host, r.RequestURI)
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
		s.logger.Printf("There was an issue writing the content to the client: %s", err.Error())
	}
}

func IsJSON(str []byte) bool {
	var js json.RawMessage
	return json.Unmarshal(str, &js) == nil
}
