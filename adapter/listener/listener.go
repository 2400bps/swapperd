package listener

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/republicprotocol/swapperd/core/request"
	"github.com/republicprotocol/swapperd/foundation"
	"github.com/rs/cors"
)

type httpRequestListener struct {
	manager      request.FundManager
	logger       foundation.Logger
	passwordHash [32]byte
	port         string
}

func NewHttpListener(manager request.FundManager, logger foundation.Logger, passwordHash [32]byte, port string) request.Listener {
	return &httpRequestListener{manager, logger, passwordHash, port}
}

// NewHttpListener creates a new http listener
func (listener *httpRequestListener) Run(doneCh <-chan struct{}, swapRequests chan<- foundation.SwapRequest, statusQueries chan<- foundation.StatusQuery) {
	reqHandler := request.NewHandler(listener.passwordHash, listener.manager, swapRequests, statusQueries)
	r := mux.NewRouter()
	r.HandleFunc("/swaps", postSwapsHandler(reqHandler)).Methods("POST")
	r.HandleFunc("/swaps", getSwapsHandler(reqHandler)).Methods("GET")
	r.HandleFunc("/transfers", postTransfersHandler(reqHandler)).Methods("POST")
	r.HandleFunc("/balances", getBalancesHandler(reqHandler)).Methods("GET")
	r.HandleFunc("/info", getInfoHandler(reqHandler)).Methods("GET")
	r.Use(recoveryHandler)
	httpHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST"},
	}).Handler(r)

	httpListener, err := net.Listen("tcp", fmt.Sprintf(":%s", listener.port))
	if err != nil {
		panic(err)
	}
	go func() {
		if err := http.Serve(httpListener, httpHandler); err != nil {
			panic(err)
		}
	}()
	listener.logger.GlobalLogInfo(fmt.Sprintf("listening for swaps on http://127.0.0.1:%s", listener.port))
	<-doneCh
	httpListener.Close()
}

// writeError response.
func writeError(w http.ResponseWriter, statusCode int, err string) {
	w.WriteHeader(statusCode)
	w.Write([]byte(err))
	return
}

// recoveryHandler handles errors while processing the requests and populates
// the errors in the response.
func recoveryHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				writeError(w, http.StatusInternalServerError, fmt.Sprintf("%v", r))
			}
		}()
		h.ServeHTTP(w, r)
	})
}

// getInfoHandler handles the get info request, it returns the basic information
// of the swapper such as the version, supported tokens addresses.
func getInfoHandler(reqHandler request.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(reqHandler.GetInfo()); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("cannot encode info response: %v", err))
			return
		}
	}
}

// getSwapsHandler handles the get swaps request, it returns the status of all
// the existing swaps on the swapper.
func getSwapsHandler(reqHandler request.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(reqHandler.GetSwaps()); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("cannot encode swaps response: %v", err))
			return
		}
	}
}

// postSwapsHandler handles the post swaps request, it fills incomplete
// information and starts the Atomic Swap.
func postSwapsHandler(reqHandler request.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, password, ok := r.BasicAuth()
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		swapReq := request.PostSwapRequest{}
		if err := json.NewDecoder(r.Body).Decode(&swapReq); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("cannot decode swap request: %v", err))
			return
		}
		swapReq.Password = password

		patchedSwap, err := reqHandler.PostSwaps(swapReq)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(patchedSwap); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("cannot encode swap response: %v", err))
			return
		}
	}
}

// postTransferHandler handles the post withdrawal request.
func postTransfersHandler(reqHandler request.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, password, ok := r.BasicAuth()
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		transferReq := request.PostTransfersRequest{}
		if err := json.NewDecoder(r.Body).Decode(&transferReq); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("cannot decode transfers request: %v", err))
			return
		}
		transferReq.Password = password

		transferResp, err := reqHandler.PostTransfers(transferReq)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("cannot decode transfers request: %v", err))
			return
		}

		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(transferResp); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("cannot encode transfers response: %v", err))
			return
		}
	}
}

// getBalancesHandler handles the get balances request, and returns the balances
// of the accounts held by the swapper.
func getBalancesHandler(reqHandler request.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		balancesRes, err := reqHandler.GetBalances()
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("cannot get balances: %v", err))
			return
		}
		if err := json.NewEncoder(w).Encode(balancesRes); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("cannot encode balances response: %v", err))
			return
		}
	}
}
