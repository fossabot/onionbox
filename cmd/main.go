package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/ciehanski/onionbox/internal/pkg/onionbox"
	"github.com/ciehanski/onionbox/internal/pkg/onionbuffer"
	"github.com/cretz/bine/tor"
	"github.com/ipsn/go-libtor"
	"github.com/natefinch/lumberjack"
)

func main() {
	ob := &onionbox.Onionbox{ // Create onionbox instance that stores config
		Version: "v0.1.0",
		Logger:  log.New(os.Stdout, "[onionbox] ", log.LstdFlags),
		Store:   onionbuffer.NewStore(),
	}

	// Init flags
	flag.BoolVar(&ob.Debug, "debug", false, "run in debug mode")
	flag.BoolVar(&ob.TorVersion3, "torv3", true, "use version 3 of the Tor circuit")
	flag.Int64Var(&ob.MaxFormMemory, "mem", 512, "max memory (mb) allotted for handling form file buffers")
	flag.Int64Var(&ob.ChunkSize, "chunks", 1024, "size of chunks for buffer I/O")
	flag.IntVar(&ob.Port, "port", 80, "port to expose the onion service on")
	flag.Parse()

	// If debug is NOT enabled, write all logs to disk (instead of stdout)
	// and rotate them when necessary.
	var torLog io.Writer
	if !ob.Debug {
		lj := &lumberjack.Logger{
			Filename:   "/var/log/onionbox/onionbox.log",
			MaxSize:    100, // megabytes
			MaxBackups: 3,
			MaxAge:     28, // days
			Compress:   true,
		}
		ob.Logger.SetOutput(lj)
		torLog = lj
	} else {
		ob.Logger.SetOutput(os.Stdout)
		torLog = os.Stderr
	}

	// Create a separate go routine which infinitely loops through the store to check for
	// expired buffer entries, and delete them.
	go func() {
		if err := ob.Store.DestroyExpiredBuffers(); err != nil {
			ob.Logf("Error destroying expired buffers: %v", err)
		}
	}()

	ob.Logf("Starting and registering onion service, please wait...")
	t, err := tor.Start(nil, &tor.StartConf{ // Start tor
		ProcessCreator:         libtor.Creator,
		DebugWriter:            torLog,
		UseEmbeddedControlConn: runtime.GOOS != "windows", // This option is not supported on Windows
		TempDataDirBase:        "/tmp",
		RetainTempDataDir:      false,
	})
	if err != nil {
		ob.Logf("Failed to start Tor: %v", err)
		ob.Quit()
	}
	defer func() {
		if err = t.Close(); err != nil {
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
		ob.Logf("Error initializing onion service: %v", err)
		ob.Quit()
	}
	defer func() {
		if err = onionSvc.Close(); err != nil {
			ob.Logf("Error closing connection to onion service: %v", err)
			ob.Quit()
		}
	}()

	// Display the onion service URL
	ob.OnionURL = onionSvc.ID
	log.Printf("Please open a Tor capable browser and navigate to http://%v.onion\n", ob.OnionURL)

	// Init serving
	http.HandleFunc("/", ob.Router)
	srv := &http.Server{
		// TODO: comeback. Tor is quite slow and depending on the size of the files being
		//  transferred, the server could timeout. I would like to keep set timeouts, but
		//  will need to find a sweet spot or enable an option for large transfers.
		IdleTimeout:  time.Minute * 3,
		ReadTimeout:  time.Minute * 3,
		WriteTimeout: time.Minute * 3,
		Handler:      nil,
	}

	// Begin serving
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(onionSvc) }()
	if err = <-errCh; err != nil {
		ob.Logf("Error serving on onion service: %v", err)
		ob.Quit()
	}

	defer func() { // Proper server shutdown when program ends
		if err = srv.Shutdown(context.Background()); err != nil {
			ob.Logf("Error shutting down onionbox server: %v", err)
			ob.Quit()
		}
	}()
}
