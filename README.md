

For correct redirection to authentik, add this to `/etc/hosts` file:
```
127.0.0.1  localhost auth.localhost.com app.localhost.com
```
The apps will be hosted on these domains.


+ Grafana is fed data metrics from Prometheus. There are some custom dashboards for Grafana in `grafana/dashboards`

+ `loadtest` contains a custom Golang script for generating traffic to the application

+ `prometheus/prometheus.yml` sets up Prometheus to listen on specific ports of different Docker containers on the local network.

+ `test_files` contains .tex and .typ files for testing.

+ `authentik/data/media/public` contains images for Authentik.

