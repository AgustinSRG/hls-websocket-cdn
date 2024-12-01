// SSL utils

package main

import (
	"crypto/tls"
	"os"
	"sync"
	"time"

	"github.com/AgustinSRG/glog"
)

// Struct to store SSL loader status
type SslCertificateLoader struct {
	logger *glog.Logger

	certPath string
	keyPath  string

	cert   *tls.Certificate
	certMu *sync.Mutex

	lastLoaded time.Time

	certModTime time.Time
	keyModTime  time.Time

	checkReloadSeconds int
}

// Creates certificate loader, loading it for the first time
func NewSslCertificateLoader(logger *glog.Logger, certPath string, keyPath string, checkReloadSeconds int) (*SslCertificateLoader, error) {
	statCert, err := os.Stat(certPath)

	if err != nil {
		return nil, err
	}

	certModTime := statCert.ModTime()

	statKey, err := os.Stat(keyPath)

	keyModTime := statKey.ModTime()

	if err != nil {
		return nil, err
	}

	cer, err := tls.LoadX509KeyPair(certPath, keyPath)

	if err != nil {
		return nil, err
	}

	return &SslCertificateLoader{
		logger:             logger,
		certPath:           certPath,
		keyPath:            keyPath,
		cert:               &cer,
		certMu:             &sync.Mutex{},
		lastLoaded:         time.Now(),
		certModTime:        certModTime,
		keyModTime:         keyModTime,
		checkReloadSeconds: checkReloadSeconds,
	}, nil
}

// Runs thread to automatically reload SSL certificates
func (loader *SslCertificateLoader) RunReloadThread() {
	for {
		// Wait some time to check
		time.Sleep(time.Duration(loader.checkReloadSeconds) * time.Second)

		// Check mod times

		statCert, err := os.Stat(loader.certPath)

		if err != nil {
			loader.logger.Errorf("Error loading SSL certificate: %v", err)
			continue
		}

		certModTime := statCert.ModTime()

		statKey, err := os.Stat(loader.keyPath)

		keyModTime := statKey.ModTime()

		if err != nil {
			loader.logger.Errorf("Error loading SSL key: %v", err)
			continue
		}

		if keyModTime.UnixMilli() == loader.keyModTime.UnixMilli() && certModTime.UnixMilli() == loader.certModTime.UnixMilli() {
			// No changes
			continue
		}

		// Reload certificate

		cer, err := tls.LoadX509KeyPair(loader.certPath, loader.keyPath)

		if err != nil {
			loader.logger.Errorf("Error loading SSL key pair: %v", err)
			continue
		}

		loader.lastLoaded = time.Now()
		loader.certModTime = certModTime
		loader.keyModTime = keyModTime

		loader.certMu.Lock()

		loader.cert = &cer

		loader.certMu.Unlock()

		loader.logger.Info("Reloaded SSL certificates")
	}
}

func (loader *SslCertificateLoader) GetCertificateFunc() func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		loader.certMu.Lock()
		defer loader.certMu.Unlock()
		return loader.cert, nil
	}
}
