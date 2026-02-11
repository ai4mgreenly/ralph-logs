RALPH_DIR ?= $(HOME)/.local/state/ralph

ralph-logs: main.go index.html favicon.svg
	go build -o ralph-logs .

run: ralph-logs
	./ralph-logs 4000 \
		'$(RALPH_DIR)/logs/ralph-runs.log' \
		'$(RALPH_DIR)/clones/*/*/*/.pipeline/cache/ralph.log'

clean:
	rm -f ralph-logs
