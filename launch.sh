#!/bin/sh
./logtail 4000 \
  '/home/ai4mgreenly/projects/ikigai-5/.ralphs/mgreenly/ikigai/*/.pipeline/cache/ralph.log' \
  '/home/ai4mgreenly/projects/ikigai-5/.pipeline/cache/orchestrator.log'
