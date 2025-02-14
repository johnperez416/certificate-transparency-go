// Copyright 2021 Google LLC. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package impl is the implementation of the witness server.
package impl

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/golang/glog"
	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/internal/witness/api"
	ih "github.com/google/certificate-transparency-go/internal/witness/cmd/witness/internal/http"
	"github.com/google/certificate-transparency-go/internal/witness/cmd/witness/internal/witness"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3" // Load drivers for sqlite3
)

// LogConfig contains a list of LogInfo (configuration options for a log).
type LogConfig struct {
	Logs []LogInfo `yaml:"Logs"`
}

// LogInfo contains the configuration options for a log, which is just its public key.
type LogInfo struct {
	PubKey string `yaml:"PubKey"`
}

// ServerOpts provides the options for a server (specified in main.go).
type ServerOpts struct {
	// Where to listen for requests.
	ListenAddr string
	// The file for sqlite3 storage.
	DBFile string
	// The signing key for the witness.
	PrivKey string
	// The log configuration information.
	Config LogConfig
}

// buildLogMap loads the log configuration information into a map.
func buildLogMap(config LogConfig) (map[string]ct.SignatureVerifier, error) {
	logMap := make(map[string]ct.SignatureVerifier)
	for _, log := range config.Logs {
		// Use the PubKey string to first create the verifier.
		pk, err := ct.PublicKeyFromB64(log.PubKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create public key: %v", err)
		}
		logV, err := ct.NewSignatureVerifier(pk)
		if err != nil {
			return nil, fmt.Errorf("failed to create signature verifier: %v", err)
		}
		// And then to create the (alphanumeric) logID.
		logID, err := api.LogIDFromPubKey(log.PubKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create log id: %v", err)
		}
		logMap[logID] = *logV
	}
	return logMap, nil
}

// Main sets up and runs the witness given the options.
func Main(ctx context.Context, opts ServerOpts) error {
	if len(opts.DBFile) == 0 {
		return errors.New("DBFile is required")
	}
	// Start up local database.
	glog.Infof("Connecting to local DB at %q", opts.DBFile)
	db, err := sql.Open("sqlite3", opts.DBFile)
	if err != nil {
		return fmt.Errorf("failed to connect to DB: %w", err)
	}
	// Load log configuration into the map.
	logMap, err := buildLogMap(opts.Config)
	if err != nil {
		return fmt.Errorf("failed to load configurations: %v", err)
	}

	w, err := witness.New(witness.Opts{
		DB:        db,
		PrivKey:   opts.PrivKey,
		KnownLogs: logMap,
	})
	if err != nil {
		return fmt.Errorf("error creating witness: %v", err)
	}

	glog.Infof("Starting witness server...")
	srv := ih.NewServer(w)
	r := mux.NewRouter()
	srv.RegisterHandlers(r)
	hServer := &http.Server{
		Addr:    opts.ListenAddr,
		Handler: r,
	}
	e := make(chan error, 1)
	go func() {
		e <- hServer.ListenAndServe()
		close(e)
	}()
	<-ctx.Done()
	glog.Info("Server shutting down")
	hServer.Shutdown(ctx)
	return <-e
}
