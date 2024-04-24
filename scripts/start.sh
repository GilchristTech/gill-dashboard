#!/bin/bash
cd "$(cd $(dirname $0)/../ && pwd)"
nodemon -e go,mod,html -x make run --signal SIGTERM
