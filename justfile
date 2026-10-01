# Justfile

rootdir := "/home/wojtekp/Programming/tex-typst-rendering-cluster/"
staticdir := rootdir + "goworkspace/restapi/static/"

# A local `dc` shortcut for all docker compose files
# dc := "docker compose -f docker-compose.app.yml -f docker-compose.auth.yml -f docker-compose.proxy.yml -f docker-compose.databases.yml -f docker-compose.monitoring.yml -f docker-compose.storage.yml -f docker-compose.volumes.yml"

# Compile test.tex to PDF via the pandoc container, clean aux files
convpdf:
    docker compose run --rm pandoc sh -c "pdflatex --interaction=nonstopmode test.tex test.pdf ; latexmk -c"

# Convert test.md to test2.pdf
convmd:
    docker compose run --rm pandoc sh -c "pandoc test.md -o test2.pdf"

# Convert test.tex to test.html
conv-tex-html:
    docker compose run --rm pandoc sh -c "pandoc test.tex -o test.html"

# Build the load testing binary
build-loadtest:
    cd loadtest && go build -o loadtest loadtest

# --- Load tests ---

# 6000 chars, LaTeX, PDF
run-loadtest-6000-pdf-latex:
    ./loadtest/loadtest --length=6000 --engine=latex --format=pdf

# 6000 chars, LaTeX, PNG
run-loadtest-6000-png-latex:
    ./loadtest/loadtest --length=6000 --engine=latex --format=png

# 6000 chars, Typst, PDF
run-loadtest-6000-pdf-typst:
    ./loadtest/loadtest --length=6000 --engine=typst --format=pdf

# 6000 chars, Typst, PNG
run-loadtest-6000-png-typst:
    ./loadtest/loadtest --length=6000 --engine=typst --format=png

# Run the restapi locally (env vars sourced from goworkspace/restapi.env)
# run-restapi:
#     (export `grep -v '^#' goworkspace/restapi.env | xargs`; cd goworkspace/restapi && go run .)

# Open an interactive redis-cli against the running redis container
redis-cli:
    docker exec -it redis redis-cli

# --- Docker compose shortcuts ---

# # Run `docker compose <args>` with all the compose files
# dc *args:
#     {{dc}} {{args}}

# # Build and start all docker compose services
# dcub:
#     {{dc}} up --build

# Alias: `just dcu` == `just dcub`
# alias dcu := dcub

# Open all local dev dashboards in Chromium
openwww:
    chromium --new-window http://localhost:9090 \
        http://localhost:3000 \
        http://localhost:8025 \
        http://localhost:8001 \
        http://localhost:5540 \
        http://localhost:9001 \
        http://localhost:8080 \
        http://localhost:80

# Watch and rebuild Tailwind CSS
tailwindcss:
    tailwindcss \
        -i {{staticdir + "input.css"}} \
        -o {{staticdir + "input.css"}} \
        --watch

dc where *rest: 
    docker compose -f docker-compose.yml -f docker-compose.proxy-{{where}}.yml  --env-file .env --env-file .env.secret {{rest}} 


# ============================ TERRAFORM

terraform_global_flags := "-chdir=terraform"
# terraform_local_flags := "-var-file=.env"

tf-init:
	terraform {{terraform_global_flags}} init

tf-plan:
	terraform {{terraform_global_flags}} plan    

tf-apply:
	terraform {{terraform_global_flags}} apply 

#============================== prepare envs

alias pe := prep_envs

is_local := if env_var_or_default("TEXST_LOCAL", "") != "" { "true" } else { "false" }
override_envfile :=(
    if is_local == "true"
        { "./.env.dev.raw"  }
    else
        { "./env.prod.raw"}
)

prep_envs:
    # eval raw env and dev/prod 
    set -a; source {{ override_envfile }}; source ./.env.raw; set +a; envsubst < ./.env.raw > ./.env 
    # eval raw secret env
    set -a; source ./.env.secret.raw; set +a; DOLLAR='$' envsubst < ./.env.secret.raw > .env.secret
    #prep secret TEMPLATE
    cp .env.secret .env.secret.TEMPLATE
    sd  "=.*" "=" .env.secret.TEMPLATE   
    sd  "#.*" "" .env.secret.TEMPLATE
    sd  -A "\n{2,}" "\n" .env.secret.TEMPLATE
    #prep terraform envs
    rg -I "^TF_VAR.*" .env .env.secret | sd "TF_VAR_" "" > terraform/.auto.tfvars

#===================== local certs for https

gen_certs *domains:
    mkcert \
    -key-file certs/key.pem \
    -cert-file certs/cert.pem \
    {{ domains }}

