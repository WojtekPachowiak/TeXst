convpdf:
	docker compose run --rm pandoc  sh -c "pdflatex --interaction=nonstopmode test.tex test.pdf ; latexmk -c"

convmd:
	docker compose run --rm pandoc  sh -c "pandoc test.md -o test2.pdf"


conv-tex-html:
	docker compose run --rm pandoc  sh -c "pandoc test.tex -o test.html"
