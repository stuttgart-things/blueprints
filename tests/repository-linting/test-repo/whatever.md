# Test YAML file with various linting issues
name: test-application
version: 1.0.0

# Bad indentation below (3 spaces instead of 2)
config:
   database:
     host: localhost
     port: 5432
       username: admin  # Too much indentation
     password: secret

# Trailing spaces on next line
servers:
  - name: server1
    ip: 192.168.1.1

  - name: server2
    ip: 192.168.1.2




# Too many empty lines above (4 instead of max 2)

environment:
  production:
    url: https://example.com/very/long/url/that/exceeds/the/maximum/line/length/configured/in/yamllint/and/should/trigger/a/warning
  staging:
    url: https://staging.example.com

# Mixed indentation
services:
  api:
    port: 8080
	replicas: 3  # This line uses a tab instead of spaces

# Line with trailing whitespace
features:
  - authentication
  - authorization
  - logging