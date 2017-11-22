package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"testing"

	"github.com/pkg/errors"
)

func TestTLSConn(t *testing.T) {
	conn, err := tls.Dial("tcp", "app01-tech-ebay-kr.koreacentral.cloudapp.azure.com:443", &tls.Config{})
	if err != nil {
		panic("failed to connect: " + err.Error())
	}
	defer conn.Close()

	// reader := bufio.NewReader(conn)
	// writer := bufio.NewWriter(conn)

	targetAddr := "app01-tech-ebay-kr.koreacentral.cloudapp.azure.com:22"
	conn.Write([]byte(fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", targetAddr, targetAddr)))

	reader := bufio.NewReader(conn)
	header, err := readHeader(reader)
	if err != nil {
		t.Fatal(errors.Wrap(err, "Unable to read response header"))
	}
	fmt.Println(string(header))

	res, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(header)), nil)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != http.StatusOK {
		t.Fatal(err)
	}
}
