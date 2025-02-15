/**
 * Copyright 2021 - Present Okta, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package server

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	idx "github.com/okta/okta-idx-golang"
	verifier "github.com/okta/okta-jwt-verifier-golang"
	"github.com/patrickmn/go-cache"

	"github.com/okta/samples-golang/identity-engine/embedded-sign-in-widget/config"
)

const (
	SESSION_STORE_NAME = "okta-self-hosted-session-store"
)

type Exchange struct {
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
	AccessToken      string `json:"access_token,omitempty"`
	TokenType        string `json:"token_type,omitempty"`
	ExpiresIn        int    `json:"expires_in,omitempty"`
	Scope            string `json:"scope,omitempty"`
	IdToken          string `json:"id_token,omitempty"`
}

type PKCE struct {
	CodeVerifier        string
	CodeChallenge       string
	CodeChallengeMethod string
}

type Server struct {
	config            *config.Config
	idxClient         *idx.Client
	tpl               *template.Template
	sessionStore      *sessions.CookieStore
	ViewData          ViewData
	cache             *cache.Cache
	svc               *http.Server
	address           string
	pkce              *PKCE
	state             string
	interactionHandle string
}

type ViewData map[string]interface{}

func NewServer(c *config.Config) *Server {
	idx, err := idx.NewClient()
	if err != nil {
		log.Fatalf("new client error: %+v", err)
	}

	// Generate random byte array for state parameter
	b := make([]byte, 16)
	rand.Read(b)

	return &Server{
		config:       c,
		tpl:          template.Must(template.ParseGlob("templates/*.gohtml")),
		idxClient:    idx,
		sessionStore: sessions.NewCookieStore([]byte("randomKey")),
		cache:        cache.New(5*time.Minute, 10*time.Minute),
		ViewData: map[string]interface{}{
			"Authenticated": false,
			"Errors":        "",
		},
		state: hex.EncodeToString(b),
	}
}

func (s *Server) Address() string {
	return s.address
}

func (s *Server) Run() {
	r := mux.NewRouter()
	r.Use(s.loggingMiddleware)

	r.HandleFunc("/", s.HomeHandler).Methods("GET")

	r.HandleFunc("/login", s.LoginHandler).Methods("GET")
	r.HandleFunc("/login/callback", s.LoginCallbackHandler).Methods("GET")
	r.HandleFunc("/profile", s.ProfileHandler).Methods("GET")
	r.HandleFunc("/logout", s.LogoutHandler).Methods("POST")

	addr := "localhost:8000"
	logger := log.New(os.Stderr, "http: ", log.LstdFlags)
	srv := &http.Server{
		Handler:      r,
		Addr:         addr,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		ErrorLog:     logger,
	}

	s.svc = srv
	s.address = srv.Addr

	log.Printf("running sample on addr %q\n", addr)

	if !s.config.Testing {
		log.Fatal(srv.ListenAndServe())
	} else {
		go func() {
			log.Fatal(srv.ListenAndServe())
		}()
	}
}

func (s *Server) HomeHandler(w http.ResponseWriter, r *http.Request) {
	type customData struct {
		Profile         map[string]string
		IsAuthenticated bool
	}

	data := customData{
		Profile:         s.getProfileData(r),
		IsAuthenticated: s.isAuthenticated(r),
	}

	s.tpl.ExecuteTemplate(w, "home.gohtml", data)
}

func (s *Server) LoginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "no-cache") // See https://github.com/okta/samples-golang/issues/20

	session, err := s.sessionStore.Get(r, SESSION_STORE_NAME)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	if session.Values["pkceData"] == nil || session.Values["pkceData"] == "" {
		s.pkce, err = createPKCEData()
		if err != nil {
			fmt.Printf("could not create pkce data: %s\n", err.Error())
			os.Exit(1)
		}
		session.Values["pkce_code_verifier"] = s.pkce.CodeVerifier
		session.Values["pkce_code_challenge"] = s.pkce.CodeChallenge
		session.Values["pkce_code_challenge_method"] = s.pkce.CodeChallengeMethod
		session.Save(r, w)
	} else {
		s.pkce.CodeVerifier = session.Values["pkce_code_verifier"].(string)
		s.pkce.CodeChallenge = session.Values["pkce_code_challenge"].(string)
		s.pkce.CodeChallengeMethod = session.Values["pkce_code_challenge_method"].(string)
	}
	nonce, err := generateNonce()
	if err != nil {
		fmt.Printf("error: %s\n", err.Error())
		os.Exit(1)
	}
	type customData struct {
		IsAuthenticated   bool
		BaseUrl           string
		ClientId          string
		Issuer            string
		State             string
		Nonce             string
		InteractionHandle string
		Pkce              *PKCE
	}

	interactionHandle, err := s.getInteractionHandle(s.pkce.CodeChallenge)
	s.interactionHandle = interactionHandle
	if err != nil {
		fmt.Printf("could not get interactionHandle: %s\n", err.Error())
	}

	issuerURL := s.idxClient.Config().Okta.IDX.Issuer
	issuerParts, err := url.Parse(issuerURL)
	if err != nil {
		fmt.Printf("error: %s\n", err.Error())
		os.Exit(1)
	}
	baseUrl := issuerParts.Scheme + "://" + issuerParts.Hostname()

	data := customData{
		IsAuthenticated:   s.isAuthenticated(r),
		BaseUrl:           baseUrl,
		ClientId:          s.idxClient.Config().Okta.IDX.ClientID,
		Issuer:            s.idxClient.Config().Okta.IDX.Issuer,
		State:             s.state,
		Nonce:             nonce,
		Pkce:              s.pkce,
		InteractionHandle: interactionHandle,
	}
	err = s.tpl.ExecuteTemplate(w, "login.gohtml", data)
	if err != nil {
		fmt.Printf("error: %s\n", err.Error())
	}
}

func (s *Server) LoginCallbackHandler(w http.ResponseWriter, r *http.Request) {
	// Check the state that was returned in the query string is the same as the above state
	if r.URL.Query().Get("state") != s.state {
		fmt.Fprintln(w, "The state was not as expected")
		return
	}

	// Check if interaction_required error is returned
	if r.URL.Query().Get("error") == "interaction_required" {
		w.Header().Add("Cache-Control", "no-cache")

		// render the widget with the saved interaction handle
		type customData struct {
			IsAuthenticated   bool
			BaseUrl           string
			ClientId          string
			Issuer            string
			State             string
			InteractionHandle string
			Pkce              *PKCE
		}

		issuerURL := s.idxClient.Config().Okta.IDX.Issuer
		issuerParts, err := url.Parse(issuerURL)
		if err != nil {
			fmt.Printf("error: %s\n", err.Error())
			os.Exit(1)
		}
		baseUrl := issuerParts.Scheme + "://" + issuerParts.Hostname()

		data := customData{
			IsAuthenticated:   s.isAuthenticated(r),
			BaseUrl:           baseUrl,
			ClientId:          s.idxClient.Config().Okta.IDX.ClientID,
			Issuer:            s.idxClient.Config().Okta.IDX.Issuer,
			State:             s.state,
			Pkce:              s.pkce,
			InteractionHandle: s.interactionHandle,
		}
		err = s.tpl.ExecuteTemplate(w, "login.gohtml", data)
		if err != nil {
			fmt.Printf("error: %s\n", err.Error())
		}
		return
	}

	// Make sure the interaction_code was provided
	if r.URL.Query().Get("interaction_code") == "" {
		fmt.Fprintln(w, "The interaction_code was not returned or is not accessible")
		return
	}

	session, err := s.sessionStore.Get(r, SESSION_STORE_NAME)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	if session.Values["pkce_code_verifier"] == nil ||
		session.Values["pkce_code_verifier"] == "" ||
		session.Values["pkce_code_challenge"] == nil ||
		session.Values["pkce_code_challenge"] == "" ||
		session.Values["pkce_code_challenge_method"] == nil ||
		session.Values["pkce_code_challenge_method"] == "" {
		fmt.Fprintln(w, "Could not get PKCE Data from session")
		return
	}
	q := r.URL.Query()
	q.Del("state")

	q.Add("grant_type", "interaction_code")
	q.Set("interaction_code", r.URL.Query().Get("interaction_code"))
	q.Add("client_id", s.idxClient.Config().Okta.IDX.ClientID)
	q.Add("client_secret", s.idxClient.Config().Okta.IDX.ClientSecret)
	q.Add("code_verifier", session.Values["pkce_code_verifier"].(string))

	url := s.oAuthEndPoint(fmt.Sprintf("token?%s", q.Encode()))
	req, _ := http.NewRequest("POST", url, bytes.NewReader([]byte("")))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: time.Second * 30}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("RESP ERROR: %+v\n", err.Error())
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("READ ERROR: %+v\n", err.Error())
	}
	defer resp.Body.Close()

	var exchange Exchange
	err = json.Unmarshal(body, &exchange)
	if err != nil {
		log.Fatalf("UNMARSHAL ERROR: %+v\n", err.Error())
	}

	_, verificationError := s.verifyToken(exchange.IdToken)

	if verificationError != nil {
		log.Fatalf("Verification Error: %+v\n", verificationError)
	}

	s.cache.Add(fmt.Sprintf("%s-id_token", session.ID), exchange.IdToken, time.Hour)
	s.cache.Add(fmt.Sprintf("%s-access_token", session.ID), exchange.AccessToken, time.Hour)

	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) ProfileHandler(w http.ResponseWriter, r *http.Request) {
	type customData struct {
		Profile         map[string]string
		IsAuthenticated bool
	}

	data := customData{
		Profile:         s.getProfileData(r),
		IsAuthenticated: s.isAuthenticated(r),
	}
	s.tpl.ExecuteTemplate(w, "profile.gohtml", data)
}

func (s *Server) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// revoke the oauth2 access token it exists in the session API side before flushing cache
	if session, err := s.sessionStore.Get(r, SESSION_STORE_NAME); err != nil {
		if accessToken, found := s.cache.Get(fmt.Sprintf("%s-access_token", session.ID)); found {
			revokeTokenUrl := s.oAuthEndPoint("revoke")
			form := url.Values{}
			form.Set("token", accessToken.(string))
			form.Set("token_type_hint", "access_token")
			form.Add("client_id", s.idxClient.Config().Okta.IDX.ClientID)
			form.Add("client_secret", s.idxClient.Config().Okta.IDX.ClientSecret)
			req, _ := http.NewRequest("POST", revokeTokenUrl, strings.NewReader(form.Encode()))
			h := req.Header
			h.Add("Accept", "application/json")
			h.Add("Content-Type", "application/x-www-form-urlencoded")

			client := &http.Client{Timeout: time.Second * 30}
			resp, err := client.Do(req)
			if err != nil {
				body, _ := ioutil.ReadAll(resp.Body)
				fmt.Printf("revoke error; status: %s, body: %s\n", resp.Status, string(body))
			}
			defer resp.Body.Close()
		}
	}

	s.cache.Flush()
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("DEBUG") == "true" || !s.config.Testing {
			log.Printf("%s: %s\n", r.Method, r.RequestURI)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) verifyToken(t string) (*verifier.Jwt, error) {
	tv := map[string]string{}
	tv["aud"] = s.idxClient.Config().Okta.IDX.ClientID
	jv := verifier.JwtVerifier{
		Issuer:           s.idxClient.Config().Okta.IDX.Issuer,
		ClaimsToValidate: tv,
	}

	result, err := jv.New().VerifyIdToken(t)
	if err != nil {
		return nil, fmt.Errorf("%s; token: %s", err, t)
	}

	if result != nil {
		return result, nil
	}

	return nil, fmt.Errorf("token could not be verified: %s", "")
}

func (s *Server) getProfileData(r *http.Request) map[string]string {
	m := make(map[string]string)

	session, err := s.sessionStore.Get(r, SESSION_STORE_NAME)

	accessToken := session.Values["access_token"]
	if accessToken == nil || accessToken == "" {
		accessToken, _ = s.cache.Get(fmt.Sprintf("%s-access_token", session.ID))
	}
	if err != nil || accessToken == nil || accessToken == "" {
		return m
	}

	reqUrl := s.oAuthEndPoint("userinfo")
	req, _ := http.NewRequest("GET", reqUrl, bytes.NewReader([]byte("")))
	h := req.Header
	h.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	h.Add("Accept", "application/json")

	client := &http.Client{Timeout: time.Second * 30}
	resp, _ := client.Do(req)
	body, _ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	json.Unmarshal(body, &m)

	return m
}

func (s *Server) isAuthenticated(r *http.Request) bool {
	session, err := s.sessionStore.Get(r, SESSION_STORE_NAME)
	idToken := session.Values["id_token"]
	if idToken == nil || idToken == "" {
		idToken, _ = s.cache.Get(fmt.Sprintf("%s-id_token", session.ID))
	}
	if err != nil || idToken == nil || idToken == "" {
		return false
	}

	return true
}

// Creates a codeVerifier that is used for PKCE
func createCodeVerifier() (*string, error) {
	codeVerifier := make([]byte, 86)
	_, err := rand.Read(codeVerifier)
	if err != nil {
		return nil, fmt.Errorf("error creating code_verifier: %w", err)
	}

	s := base64.RawURLEncoding.EncodeToString(codeVerifier)
	return &s, nil
}

// Create the PKCE data for the authentication flow.
// This data will be used when getting an interaction
// handle as well as when you exchange your tokens.
func createPKCEData() (*PKCE, error) {
	h := sha256.New()

	codeVerifier, err := createCodeVerifier()
	if err != nil {
		return nil, fmt.Errorf("failed to create codeVerifier: %w", err)
	}

	_, err = h.Write([]byte(*codeVerifier))
	if err != nil {
		return nil, fmt.Errorf("failed to write codeVerifier: %w", err)
	}

	codeChallenge := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return &PKCE{
		CodeChallenge:       codeChallenge,
		CodeVerifier:        *codeVerifier,
		CodeChallengeMethod: "S256",
	}, nil
}

// Generate a Nonce to be used during the initialization of the SIW
func generateNonce() (string, error) {
	nonceBytes := make([]byte, 32)
	_, err := rand.Read(nonceBytes)
	if err != nil {
		return "", fmt.Errorf("could not generate nonce")
	}

	return base64.URLEncoding.EncodeToString(nonceBytes), nil
}

// Get the interaction handle to begin the flow. Use this
// value when initializing the Okta sign in widget.
func (s *Server) getInteractionHandle(codeChallenge string) (string, error) {

	data := url.Values{}
	data.Set("scope", strings.Join(s.idxClient.Config().Okta.IDX.Scopes, " "))
	data.Set("code_challenge", codeChallenge)
	data.Set("code_challenge_method", "S256")
	data.Set("redirect_uri", s.idxClient.Config().Okta.IDX.RedirectURI)
	data.Set("state", s.state)

	endpoint := s.oAuthEndPoint("interact")
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create interact http request: %w", err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: time.Second * 30}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http call has failed: %w", err)
	}
	type interactionHandleResponse struct {
		InteractionHandle string `json:"interaction_handle"`
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("READ ERROR: %+v\n", err.Error())
	}
	defer resp.Body.Close()
	var interactionHandle interactionHandleResponse
	err = json.Unmarshal(body, &interactionHandle)
	if err != nil {
		return "", err
	}

	return interactionHandle.InteractionHandle, nil
}

func (s *Server) oAuthEndPoint(operation string) string {
	var endPoint string
	issuer := s.idxClient.Config().Okta.IDX.Issuer
	if strings.Contains(issuer, "oauth2") {
		endPoint = fmt.Sprintf("%s/v1/%s", issuer, operation)
	} else {
		endPoint = fmt.Sprintf("%s/oauth2/v1/%s", issuer, operation)
	}
	return endPoint
}
