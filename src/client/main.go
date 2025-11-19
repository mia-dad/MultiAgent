package main

import (
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "os/exec"
    "runtime"
    "sync"

    "github.com/creack/pty"
)

// TerminalSession 表示一个持久会话
type TerminalSession struct {
    ID   string
    Cmd  *exec.Cmd
    PTY  *os.File      // Unix/Mac 使用 PTY
    Stdin io.WriteCloser // Windows 使用管道
    Stdout io.ReadCloser
    Buf  []byte
    Lock sync.Mutex
}

var sessions = make(map[string]*TerminalSession)
var sessionsLock = sync.Mutex{}

// 创建新 session
func newSessionHandler(w http.ResponseWriter, r *http.Request) {
    id := fmt.Sprintf("%d", len(sessions)+1)
    var cmd *exec.Cmd
    var f *os.File
    var stdin io.WriteCloser
    var stdout io.ReadCloser
    var err error

    if runtime.GOOS == "windows" {
        // Windows 用 powershell
        cmd = exec.Command("powershell.exe")
        stdin, err = cmd.StdinPipe()
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        stdout, err = cmd.StdoutPipe()
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        err = cmd.Start()
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
    } else {
        // Mac/Linux 用 PTY
        shell := os.Getenv("SHELL")
        if shell == "" {
            shell = "bash"
        }
        cmd = exec.Command(shell)
        f, err = pty.Start(cmd)
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
    }

    session := &TerminalSession{
        ID:   id,
        Cmd:  cmd,
        PTY:  f,
        Stdin: stdin,
        Stdout: stdout,
        Buf:  []byte{},
    }

    // 持续读取输出
    go func(s *TerminalSession) {
        buf := make([]byte, 1024)
        for {
            var n int
            var err error
            if runtime.GOOS == "windows" {
                n, err = s.Stdout.Read(buf)
            } else {
                n, err = s.PTY.Read(buf)
            }
            if err != nil {
                return
            }
            s.Lock.Lock()
            s.Buf = append(s.Buf, buf[:n]...)
            s.Lock.Unlock()
        }
    }(session)

    sessionsLock.Lock()
    sessions[id] = session
    sessionsLock.Unlock()

    json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// 执行命令
func execHandler(w http.ResponseWriter, r *http.Request) {
    type Req struct {
        ID      string `json:"id"`
        Command string `json:"command"`
    }
    var req Req
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), 400)
        return
    }

    sessionsLock.Lock()
    session, ok := sessions[req.ID]
    sessionsLock.Unlock()
    if !ok {
        http.Error(w, "invalid session", 400)
        return
    }

    session.Lock.Lock()
    defer session.Lock.Unlock()

    cmd := req.Command + "\n"
    var err error
    if runtime.GOOS == "windows" {
        _, err = session.Stdin.Write([]byte(cmd))
    } else {
        _, err = session.PTY.Write([]byte(cmd))
    }
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    w.Write([]byte("ok"))
}

// 获取输出
func outputHandler(w http.ResponseWriter, r *http.Request) {
    id := r.URL.Query().Get("id")
    sessionsLock.Lock()
    session, ok := sessions[id]
    sessionsLock.Unlock()
    if !ok {
        http.Error(w, "invalid session", 400)
        return
    }

    session.Lock.Lock()
    out := session.Buf
    session.Buf = []byte{} // 清空已读取
    session.Lock.Unlock()

    w.Write(out)
}

func main() {
    http.HandleFunc("/session/new", newSessionHandler)
    http.HandleFunc("/session/exec", execHandler)
    http.HandleFunc("/session/output", outputHandler)

    log.Println("Multi-session Terminal Agent started on :1234")
    log.Fatal(http.ListenAndServe(":1234", nil))
}
