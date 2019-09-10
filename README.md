reject-proxy
------------

This is a blocking MITM proxy that is configurable via a config map in a Kubernetes cluster. It can be injected as a sidecar and controls http/https traffic via `http_proxy` and `https_proxy` environment variables.

Configuration example:
```
apiVersion: v1
kind: ConfigMap
metadata:
  name: reject-proxy
data:
  config: |-
    blockedUrls:
    - name: neutron-lbaas
      method: "POST"
      url: "https://whatever.com:443/v2.0/lbaas.*"
```

## License
This project is licensed under the Apache2 License - see the [LICENSE](LICENSE) file for details
