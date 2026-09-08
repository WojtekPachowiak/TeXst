.PHONY:

convpdf:
	docker compose run --rm pandoc  sh -c "pdflatex --interaction=nonstopmode test.tex test.pdf ; latexmk -c"

convmd:
	docker compose run --rm pandoc  sh -c "pandoc test.md -o test2.pdf"

conv-tex-html:
	docker compose run --rm pandoc  sh -c "pandoc test.tex -o test.html"

build-loadtest:
	(cd loadtest && go build -o loadtest loadtest);

run-loadtest-6000-pdf-latex:
	./loadtest/loadtest --length=6000 --engine=latex --format=pdf 

run-loadtest-6000-png-latex:
	./loadtest/loadtest --length=6000 --engine=latex --format=png 

run-loadtest-6000-pdf-typst:
	./loadtest/loadtest --length=6000 --engine=typst --format=pdf 

run-loadtest-6000-png-typst:
	./loadtest/loadtest --length=6000 --engine=typst --format=png 

# run-restapi:
# 	(export `grep -v '^#' goworkspace/restapi.env | xargs`; cd goworkspace/restapi && go run .) 

redis-cli:
	docker exec -it redis redis-cli

aliasdc:
	alias dc="docker compose -f docker-compose.app.yml -f docker-compose.auth.yml -f docker-compose.proxy.yml -f docker-compose.databases.yml -f docker-compose.monitoring.yml -f docker-compose.storage.yml -f docker-compose.volumes.yml" 

dcub:
	docker compose -f docker-compose.app.yml \
				   -f docker-compose.auth.yml \
				   -f docker-compose.proxy.yml \
				   -f docker-compose.databases.yml \
				   -f docker-compose.monitoring.yml \
				   -f docker-compose.storage.yml \
				   -f docker-compose.volumes.yml \
				   up --build

openwww:
	chromium --new-window http://localhost:9090 \
			 http://localhost:3000 \
			 http://localhost:8025 \
			 http://localhost:8001 \
			 http://localhost:5540 \
			 http://localhost:9001 \
			 http://localhost:8080 \
			 http://localhost:80 \

# dcu:
# 	docker compose up