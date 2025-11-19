package main

import (
    "encoding/json"
    "log"
    "net/http"
    "os/exec"
)

type ExecRequest struct {
    Command string   `json:"command"`
    Args    []string `json:"args"`
}

type ExecResponse struct {
    Stdout   string `json:"stdout"`
    Stderr   string `json:"stderr"`
    ExitCode int    `json:"exitCode"`
    Error    string `json:"error,omitempty"`
}

func execHandler(w http.ResponseWriter, r *http.Request) {
    var req ExecRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    cmd := exec.Command(req.Command, req.Args...)

    stdoutBytes, err := cmd.Output()
    exitCode := 0

    stderrBytes := []byte{}
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            exitCode = exitErr.ExitCode()
            stderrBytes = exitErr.Stderr
        } else {
            exitCode = -1
        }
    }

    resp := ExecResponse{
        Stdout:   string(stdoutBytes),
        Stderr:   string(stderrBytes),
        ExitCode: exitCode,
    }

    if err != nil {
        resp.Error = err.Error()
    }

    json.NewEncoder(w).Encode(resp)
}

func main() {
    log.Println("Local Agent started at http://localhost:1234")

    http.HandleFunc("/exec", execHandler)
    log.Fatal(http.ListenAndServe(":1234", nil))
}
