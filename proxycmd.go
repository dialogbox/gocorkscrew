package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type HTTPProxyPipeline struct {
	ProxyAddr  string
	Scheme     string
	TargetAddr string
	TLSConfig  *tls.Config
}

func readHeader(conn io.Reader) ([]byte, error) {
	c := bufio.NewReader(conn)
	buf := make([]byte, 0, 1024)
	for {
		l, prefix, err := c.ReadLine()
		if err != nil {
			return nil, err
		}

		buf = append(buf, l...)
		buf = append(buf, '\r', '\n')
		if prefix {
			continue
		}

		if len(l) == 0 {
			return buf, nil
		}
	}
}

func (c *HTTPProxyPipeline) openConnection() (io.ReadWriteCloser, error) {
	switch c.Scheme {
	case "http":
		conn, err := net.Dial("tcp", c.ProxyAddr)
		if err != nil {
			return nil, errors.Wrap(err, "Unable to connect to the proxy server")
		}
		return conn, nil
	case "https":
		conn, err := tls.Dial("tcp", c.ProxyAddr, c.TLSConfig)
		if err != nil {
			return nil, errors.Wrap(err, "Unable to connect to the proxy server")
		}
		return conn, nil
	default:
		return nil, errors.Errorf("Unsupported scheme: %s", c.Scheme)
	}
}

func (c *HTTPProxyPipeline) Open(clientReader io.Reader, clientWriter io.Writer) error {
	conn, err := c.openConnection()
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write([]byte(fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", c.TargetAddr, c.TargetAddr)))
	if err != nil {
		return errors.Wrap(err, "Unable to write request to proxy server")
	}

	header, err := readHeader(conn)
	if err != nil {
		return errors.Wrap(err, "Unable to read response header")
	}

	res, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(header)), nil)
	if err != nil {
		return errors.Wrap(err, "Unable to parse response header")
	}

	if res.StatusCode != http.StatusOK {
		return errors.New("Bad response")
	}

	logrus.Info("Proxy connection established")

	errchan := make(chan error, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func(ctx context.Context) {
		defer logrus.Info("Proxy -> Client pipe is closed")
		select {
		case <-ctx.Done():
			return
		default:
			_, err := io.Copy(clientWriter, conn)
			if err != nil && err != io.EOF {
				errchan <- errors.Wrap(err, "Unable to transfer data from client to proxy server")
			} else {
				errchan <- nil
			}
		}
	}(ctx)

	go func(ctx context.Context) {
		defer logrus.Info("Client -> Proxy pipe is closed")
		select {
		case <-ctx.Done():
			return
		default:
			_, err := io.Copy(conn, clientReader)
			if err != nil && err != io.EOF {
				errchan <- errors.Wrap(err, "Unable to transfer data from proxy server to client")
			} else {
				errchan <- nil
			}
		}
	}(ctx)

	err = <-errchan
	if err != nil {
		return err
	}
	err = <-errchan
	if err != nil {
		return err
	}

	return nil
}

func main() {
	scheme := os.Args[1]
	proxyHost := os.Args[2]
	proxyPort := os.Args[3]
	targetHost := os.Args[4]
	targetPort := os.Args[5]

	pipe := &HTTPProxyPipeline{
		ProxyAddr:  fmt.Sprintf("%s:%s", proxyHost, proxyPort),
		Scheme:     scheme,
		TargetAddr: fmt.Sprintf("%s:%s", targetHost, targetPort),
	}

	err := pipe.Open(os.Stdin, os.Stdout)
	if err != nil {
		logrus.Error(err)
	}
}

func init() {
	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	logrus.SetOutput(os.Stderr)

	// Only log the warning severity or above.
	logrus.SetLevel(logrus.WarnLevel)
}
