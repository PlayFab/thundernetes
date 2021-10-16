# Thundernetes allocator tool

Thanks for using the allocator tool for thundernetes. To use it:
- `kubectl` is required to be in $PATH. for more information, please refer to the following [guide](https://kubernetes.io/docs/tasks/tools/#kubectl)  
- Compile the main.go file with `go build main.go` (optional to provide a meaningful name like allocator, thunderallocator or something similar).
- Once you have the executable ready, you can run it to: 
    - Provide no argument for some help and details.
    - `list` which will provide the available servers.
    - `allocate <build-id> <session-id> [tls-public] [tls-private]` where the tls certificates are optional, but build and session ID are mandatory. Please note that providing the certs as env variables is also supported; if so, please name them TLS_PUBLIC for the cert file and TLS_PRIVATE for the key file.