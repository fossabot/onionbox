package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/cretz/bine/tor"
	"github.com/ipsn/go-libtor"
	"github.com/natefinch/lumberjack"
	"onionbox/onionbox"
	"onionbox/onionbuffer"
)

func main() {
	// Create onionbox instance that stores config
	ob := &onionbox.Onionbox{
		Version: "v0.1.0",
		Logger:  log.New(os.Stdout, "[onionbox] ", log.LstdFlags),
		Store:   onionbuffer.NewStore(),
	}
	// Init flags
	flag.BoolVar(&ob.Debug, "debug", false, "run in debug mode")
	flag.BoolVar(&ob.TorVersion3, "torv3", true, "use version 3 of the Tor circuit")
	flag.Int64Var(&ob.MaxFormMemory, "mem", 512, "max memory allotted for handling form file buffers")
	flag.Int64Var(&ob.ChunkSize, "chunks", 1024, "size of chunks for buffer I/O")
	flag.IntVar(&ob.Port, "port", 80, "port to expose the onion service on")
	// Parse flags
	flag.Parse()

	// If debug is NOT enabled, write all logs to disk (instead of stdout)
	// and rotate them when necessary.
	if !ob.Debug {
		ob.Logger.SetOutput(&lumberjack.Logger{
			Filename:   "/var/log/onionbox/onionbox.log",
			MaxSize:    100, // megabytes
			MaxBackups: 3,
			MaxAge:     28, // days
			Compress:   true,
		})
	}

	// Create a separate go routine which infinitely loops through the store to check for
	// expired buffer entries, and delete them.
	go func() {
		if err := ob.Store.DestroyExpiredBuffers(); err != nil {
			ob.Logf("Error destroying expired buffers: %v", err)
		}
	}()

	// Get running OS
	var useEmbeddedCon bool
	if runtime.GOOS == "windows" {
		useEmbeddedCon = false
	} else {
		useEmbeddedCon = true
	}

	// Start tor
	ob.Logf("Starting and registering onion service, please wait...")
	t, err := tor.Start(nil, &tor.StartConf{
		ProcessCreator: libtor.Creator,
		DebugWriter:    os.Stderr,
		// This option is not supported on Windows
		UseEmbeddedControlConn: useEmbeddedCon,
	})
	if err != nil {
		ob.Logf("Failed to start Tor: %v", err)
		ob.Quit()
	}
	defer func() {
		if err := t.Close(); err != nil {
			ob.Logf("Error closing connection to Tor: %v", err)
			ob.Quit()
		}
	}()

	// Wait at most 3 minutes to publish the service
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Create an onion service to listen on any port but show as 80
	onionSvc, err := t.Listen(ctx, &tor.ListenConf{
		RemotePorts: []int{ob.Port},
		Version3:    ob.TorVersion3,
	})
	if err != nil {
		ob.Logf("Error creating the onion service: %v", err)
		ob.Quit()
	}
	defer func() {
		if err := onionSvc.Close(); err != nil {
			ob.Logf("Error closing connection to onion service: %v", err)
			ob.Quit()
		}
	}()

	// Display the onion service URL
	ob.OnionURL = onionSvc.ID
	ob.Logf("Please open a Tor capable browser and navigate to http://%v.onion\n", ob.OnionURL)

	// Init serving
	http.HandleFunc("/", ob.Router)
	srv := &http.Server{
		// TODO: comeback. Tor is quite slow and depending on the size of the files being
		//  transferred, the server could timeout. I would like to keep set timeouts, but
		//  will need to find a sweet spot or enable an option for large transfers.
		IdleTimeout:  time.Second * 60,
		ReadTimeout:  time.Minute * 5,
		WriteTimeout: time.Minute * 10,
		Handler:      nil,
	}
	// Begin serving
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(onionSvc) }()
	if err = <-errCh; err != nil {
		ob.Logf("Error serving on onion service: %v", err)
		ob.Quit()
	}
	// Proper server shutdown when program ends
	defer func() {
		if err := srv.Shutdown(context.Background()); err != nil {
			ob.Logf("Error shutting down onionbox: %v", err)
			ob.Quit()
		}
	}()
}
