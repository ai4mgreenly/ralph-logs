RALPH_DIR ?= $(HOME)/.local/state/ralph

ralph-logs: main.go index.html favicon.svg
	go build -o ralph-logs .

run: ralph-logs
	./ralph-logs \
		'$(RALPH_DIR)/logs/ralph-runs.log' \
		'$(RALPH_DIR)/clones/*/*/*/.pipeline/cache/ralph.log'

install: ralph-logs
	mkdir -p $(HOME)/.local/bin
	cp ralph-logs $(HOME)/.local/bin/ralph-logs

clean:
	rm -f ralph-logs
