package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/alecthomas/template"
	"github.com/roylee0704/honestbee/server/external"
)

var (
	tcpPort        = flag.String("tcp", ":8080", "Listen port")
	throttleLimit  = flag.Int("throttle", 30, "Throttle limit")
	throttlePeriod = flag.Duration("period", 1*time.Second, "Throttle period")
	timeoutPeriod  = flag.Duration("timeout", 15*time.Second, "External API timeout period")
)

func main() {
	s := NewServer(*throttleLimit, *throttlePeriod, *timeoutPeriod)

	fmt.Printf("TCP-Server started at port: %s\n", *tcpPort)
	s.TCPListenAndAccept(*tcpPort)
}

// SessionID uniquely identifies a tcp-connection.
// It auto-increments upon new tcp-connection.
type SessionID int
type freq struct {
	Count     int
	StartTime time.Time
}

// Server implements the TCP server.
// It serves a limited number of GitHub-issue(external API) query
// requests from TCP client.
type Server struct {
	throttle int
	period   time.Duration
	timeout  time.Duration

	sessions     map[SessionID]*freq
	curSessionID SessionID
}

// NewServer returns an initiated TCP-server.
func NewServer(throttle int, period time.Duration, timeout time.Duration) *Server {
	s := &Server{
		throttle:     throttle,
		period:       period,
		timeout:      timeout,
		sessions:     make(map[SessionID]*freq),
		curSessionID: 0,
	}
	return s
}

// TCPListenAndAccept listens and accepts client
// tcp-connections
func (s *Server) TCPListenAndAccept(port string) {
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Print(err)
			continue
		}

		s.curSessionID++
		fmt.Printf("session#%03d: connected\n", s.curSessionID)

		go s.handleConn(s.curSessionID, conn)
	}
}

// handleConn handles limited number of GitHub-issue queries
// and return results back to client.
//
// Then, it sends "bye" and drop client tcp-connection when "quit" is
// received or error.
func (s *Server) handleConn(sessionID SessionID, c net.Conn) {
	defer c.Close()

	input := bufio.NewScanner(c)
	for input.Scan() {
		query := input.Text()
		fmt.Printf("session#%03d: received request '%s'\n", s.curSessionID, query)
		if strings.ToLower(query) == "quit" {
			reply(c, "bye!")
			break
		}

		if err := rateLimiter(s.sessions, sessionID, s.throttle, s.period, time.Now()); err != nil {
			reply(c, fmt.Sprintf("error: search query failed: %s", err))
			continue
		}

		fmt.Println(s.sessions)
		result, err := external.SearchIssues(strings.Split(query, ","), s.timeout)
		if err != nil {
			reply(c, fmt.Sprintf("error: search query failed: 50x External API Error: %s", err))
			break
		}

		var tpl bytes.Buffer
		if err := report.Execute(&tpl, result); err != nil {
			reply(c, fmt.Sprintf("error: search query failed: 500 Internal Server Error: %s", err))
			break
		}

		fmt.Printf("session#%03d: returned search results for '%s'\n", s.curSessionID, query)
		reply(c, tpl.String())
	}

	fmt.Printf("session#%03d: disconnected\n", s.curSessionID)
}

// rateLimiter throttles number of requests a client(sessionID) can
// send to external API within a period. e.g: 30 requests per second.
//
// A fixed-window algorithm is used.
func rateLimiter(sessions map[SessionID]*freq, sessionID SessionID, throttle int, period time.Duration, now time.Time) error {
	r, ok := sessions[sessionID]
	if !ok {
		sessions[sessionID] = &freq{Count: 1, StartTime: now}
	} else {
		d := now.Sub(r.StartTime)
		if d > period {
			r.Count = 1
			r.StartTime = now
		} else {
			if r.Count >= throttle {
				return errors.New("403 Forbidden")
			}
			r.Count++
		}
	}
	return nil
}

// reply sends message to c tcp connection.
func reply(c net.Conn, message string) {
	fmt.Fprintln(c, message)
}

var report = template.Must(template.New("report").Parse(`
{{.TotalCount}} issues:
{{range .Items}}---------------------------
Number: {{.Number}}
User: {{.User.Login}}
Title: {{.Title }}
URL: {{.HTMLURL }}
{{end}}
`))
