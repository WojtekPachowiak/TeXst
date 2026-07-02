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

dcub:
	docker compose up --build


dcu:
	docker compose up