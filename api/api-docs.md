```http
POST /session/exec HTTP/1.1
Host:localhost:1234
Content-Type: application/json

{
    "id":"1",
    "command": "cd ~",
     "args": ["-l"]
}
```

```http
GET /session/output?id=1 HTTP/1.1
Host:localhost:1234
```
