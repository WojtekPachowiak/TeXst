.PHONY:

convpdf:
	docker compose run --rm pandoc  sh -c "pdflatex --interaction=nonstopmode test.tex test.pdf ; latexmk -c"

convmd:
	docker compose run --rm pandoc  sh -c "pandoc test.md -o test2.pdf"

conv-tex-html:
	docker compose run --rm pandoc  sh -c "pandoc test.tex -o test.html"

build-loadtest:
	(cd loadtest && go build -o loadtest loadtest);

run-loadtest:
	./loadtest/loadtest

redis-cli:
	docker exec -it redis redis-cli

hello:
	echo 

