RALPH_LOGS ?= /home/ai4mgreenly/projects/ikigai-5/

ralph-logs: main.go
	go build -o ralph-logs .

run: ralph-logs
	./ralph-logs 4000 \
		'$(RALPH_LOGS)/.ralphs/mgreenly/ikigai/*/.pipeline/cache/ralph.log' \
		'$(RALPH_LOGS)/.pipeline/cache/orchestrator.log'

clean:
	rm -f ralph-logs
