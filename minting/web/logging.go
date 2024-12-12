// Rudimentary web logging support.
package web

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	logMutex sync.Mutex
	writer   *bufio.Writer
)

func init() {
	writer = bufio.NewWriter(os.Stdout)
}

func SetWeblogOutput(filePath string) error {
	f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0664)
	if err != nil {
		return err
	}
	writer = bufio.NewWriter(f)
	return nil
}

// logRequest prints requests to the web log. Thread safe.  TODO: Use a real http logging library.
func LogRequest(r *http.Request, status int, forLog *string, username *string) {
	t := time.Now().Format(time.RFC3339)
	logMutex.Lock()
	defer logMutex.Unlock()
	fmt.Fprint(writer, t, ",", status, ",", r.RemoteAddr, ",", r.Host, ",", cleanURIForLog(r.URL.RequestURI()), ",", cleanURIForLog(r.Referer()), ",")
	if forLog != nil {
		// TODO: sanatize this string
		fmt.Fprint(writer, *forLog)
	}
	if username != nil {
		fmt.Fprint(writer, ",", *username, "\n")
	} else {
		fmt.Fprint(writer, ",\n")
	}
	writer.Flush()
}

// cleanURIForLog makes sure the given URI string is of a "safe format" to be put into the log file.
func cleanURIForLog(URI string) string {
	// Commas would confuse any log parsing since we use CSV format.
	return strings.Replace(URI, ",", "&#44;", -1)
}
