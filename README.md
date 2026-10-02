
# TeXst TODO README IN PROGRESS

<img width="1428" height="1075" alt="image" src="https://github.com/user-attachments/assets/5b41f8b3-7923-45ee-b612-455855fbbae8" />

<img width="1955" height="1126" alt="image" src="https://github.com/user-attachments/assets/9c7eb535-568c-43c1-9f8b-5f0364b5dc04" />


An app for rendering LaTeX and Typst files in the browser.

# Usage


1. You need to setup Github and Google OAuth clients.
2. change /etc/hosts

Run
```
docker compose up
```
To setup `authentik` (the login screen) you need to run Terraform:
```
cd terraform
terraform apply
```
App is ready to use



# TODO 

For correct redirection to authentik, add this to `/etc/hosts` file:
```
127.0.0.1  localhost auth.texst.wojciechpachowiak.com app.texst.wojciechpachowiak.com
```
The apps will be hosted on these domains.




+ Grafana is fed data metrics from Prometheus. There are some custom dashboards for Grafana in `grafana/dashboards`

+ `loadtest` contains a custom Golang script for generating traffic to the application

+ `prometheus/prometheus.yml` sets up Prometheus to listen on specific ports of different Docker containers on the local network.

+ `test_files` contains .tex and .typ files for testing.

+ `authentik/data/media/public` contains images for Authentik.

