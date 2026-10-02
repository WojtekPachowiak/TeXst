# Justfile

rootdir := "/home/wojtekp/Programming/tex-typst-rendering-cluster/"
staticdir := rootdir + "goworkspace/restapi/static/"

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

# Open an interactive redis-cli against the running redis container
redis-cli:
    docker exec -it redis redis-cli


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

# ================ docker

dc where *rest: 
    docker compose -f docker-compose.yml -f docker-compose.proxy-{{where}}.yml  --env-file .env.{{where}} --env-file .env.secret {{rest}} 

#=================== VPS manage

ssh-conn  := "ovhvps"

vps-prepssh:
    eval "$(ssh-agent -s)"
    ssh-add ~/.ssh/ovhvps_ed25519

vps-deploy:
    just prep_envs
    rsync -avz traefik prometheus grafana authentik {{ssh-conn}}:/home/app/
    DOCKER_HOST="ssh://{{ssh-conn}}" just dc prod up --build
    # DOCKER_HOST="ssh://{{ssh-conn}}" docker image prune -f

vps-customcmd *cmd:
    DOCKER_HOST="ssh://{{ssh-conn}}" just dc prod {{cmd}}
      

vps-hardreset:
    just prep_envs
    DOCKER_HOST="ssh://{{ssh-conn}}" just dc prod down -v
    # DOCKER_HOST="ssh://{{ssh-conn}}" docker image prune -f


# ============================ TERRAFORM

terraform_global_flags := "-chdir=terraform"
# terraform_local_flags := "-var-file=.env"

tf-init where="dev":
	terraform {{terraform_global_flags}} init -var-file=.env.tf.{{where}}

tf-plan where="dev":
	terraform {{terraform_global_flags}} plan -var-file=.env.tf.{{where}}   

tf-apply where="dev":
	terraform {{terraform_global_flags}} apply -var-file=.env.tf.{{where}}

#============================== prepare envs

alias pe := prep_envs

prep_envs:
    # eval raw env and dev/prod 
    set -a; source ./.env.dev.raw; source ./.env.raw; set +a; envsubst < ./.env.raw > ./.env.dev 
    set -a; source ./.env.prod.raw; source ./.env.raw; set +a; envsubst < ./.env.raw > ./.env.prod 
    # eval raw secret env
    set -a; source ./.env.secret.raw; set +a; DOLLAR='$' envsubst < ./.env.secret.raw > .env.secret
    chmod 600 .env.secret
    #prep secret TEMPLATE
    cp .env.secret .env.secret.TEMPLATE
    sd  "=.*" "=" .env.secret.TEMPLATE   
    sd  "#.*" "" .env.secret.TEMPLATE
    sd  -A "\n{2,}" "\n" .env.secret.TEMPLATE
    #prep terraform envs
    rg -I "^TF_VAR.*" .env.dev .env.secret | sd "TF_VAR_" "" > terraform/.env.tf.dev
    rg -I "^TF_VAR.*" .env.prod .env.secret | sd "TF_VAR_" "" > terraform/.env.tf.prod

#===================== local certs for https

gen_certs *domains:
    mkcert \
    -key-file certs/key.pem \
    -cert-file certs/cert.pem \
    {{ domains }}

