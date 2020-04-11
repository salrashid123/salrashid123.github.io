package docker.authz

default allow = false
users = {
    "user1@domain.com": {"readOnly": true},
    "user2@domain.com": {"readOnly": false},    
}

allow {
    users[input.User].readOnly
    input.Method == "GET"
}
