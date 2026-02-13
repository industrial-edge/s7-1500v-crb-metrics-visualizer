# Installation

- [Installation](#installation)
  - [Build application](#build-application)
    - [Download Repository](#download-repository)
    - [Build docker image](#build-docker-image)
  - [Configure S7-1500V Connections](#configure-s7-1500v-connections)
    - [Create Configuration File](#create-configuration-file)
    - [Configuration Format](#configuration-format)
    - [Multiple S7-1500V Setup](#multiple-s7-1500v-setup)
  - [Deploy Application](#deploy-application)
    - [Start Services](#start-services)
    - [Verify Deployment](#verify-deployment)
  - [Access Dashboard](#access-dashboard)
    - [Initial Login](#initial-login)
    - [Dashboard Configuration](#dashboard-configuration)
    - [S7-1500V Selection](#s7-1500v-selection)
  - [Proxy Configuration](#proxy-configuration)
    - [Docker Daemon Configuration](#docker-daemon-configuration)
    - [Compose Build Configuration](#compose-build-configuration)

## Build application

### Download Repository

Download or clone the repository source code to your workstation.

* Through terminal:
```bash
git clone <your-repository-url>
cd s7-1500v-crb-metrics-visualizer
```

* Through VSCode:  
<kbd>CTRL</kbd>+<kbd>⇧ SHIFT</kbd>+<kbd>P</kbd> or <kbd>F1</kbd> to open VSCode's command palette and type `git clone`

### Build docker image

- Navigate to the project root folder (where the `compose.yaml` file is located)
- The collector service will be built automatically during the first startup
- Use the following command to build all services:

```bash
docker-compose build
```

- Verify the images are created:
```bash
docker images
```

You should see images for:
- `vplc_collector` (your custom Go application)
- `prom/prometheus`
- `ghcr.io/credativ/plutono`

## Configure S7-1500V Connections

### Create Configuration File

Before starting the application, you must configure your S7-1500V connection details:

1. Copy the example configuration file:
```bash
cp cfg-data/secrets-example.json cfg-data/secrets.json
```

2. Edit the configuration file with your S7-1500V details:
```bash
# Use your preferred editor
nano cfg-data/secrets.json
# or
code cfg-data/secrets.json
```

### Configuration Format

The `secrets.json` file should contain your S7-1500V connection information:

```json
{
	"vplcs": [
		{
			"name": "S7-1500V-Production",
			"loginUrl": "https://192.168.1.1/device/edge/api/v2/login/direct",
			"apiUrl": "https://192.168.1.1/1517v/api/v2",
			"user": "username",
			"password": "password"
		}
	]
}
```

**Configuration Parameters:**
- `name`: Friendly name for the S7-1500V instance (displayed in dashboard)
- `loginUrl`: Authentication endpoint URL for S7-1500V login
- `apiUrl`: S7-1500V API endpoint for data collection. Replace the variant identifier in the URL path according to your PLC type:
  
  | PLC Variant | URL Path Identifier |
  |-------------|---------------------|
  | S7-1517V | `1517v` |
  | S7-1517VF | `1517vf` |
  
  Example: For S7-1517V, use `https://{ip-addr}/1517v/api/v2`
  
- `user`: Authentication username (typically email format)
- `password`: Authentication password

### Multiple S7-1500V Setup

To monitor multiple S7-1500V installations, add additional entries to the `vplcs` array:

```json
{
	"vplcs": [
		{
			"name": "S7-1517V-Line1",
			"loginUrl": "https://{ip-addr}/device/edge/api/v2/login/direct",
			"apiUrl": "https://{ip-addr}/1517v/api/v2",
			"user": "username",
			"password": "password"
		},
		{
			"name": "S7-1517VF-Line2",
			"loginUrl": "https://{ip-addr}/device/edge/api/v2/login/direct",
			"apiUrl": "https://{ip-addr}/1517vf/api/v2",
			"user": "another_username",
			"password": "another_password"
		}
	]
}
```

## Deploy Application

### Start Services

1. **Start all services in detached mode:**
```bash
docker-compose up -d
```

2. **Monitor startup logs:**
```bash
docker-compose logs -f
```

3. **Check service status:**
```bash
docker-compose ps
```

All services should show "Up" status:
- `collector` - S7-1500V data collection service
- `prometheus` - Time-series database
- `plutono` - Visualization dashboard

### Verify Deployment

1. **Check service status:**
```bash
docker-compose ps
```

Expected output:
```
   Name                 Command               State                    Ports
----------------------------------------------------------------------------------------------
collector    collector                        Up
plutono      /run.sh                          Up      0.0.0.0:3000->3000/tcp,:::3000->3000/tcp
prometheus   /bin/prometheus --config.f ...   Up      9090/tcp
```

2. **Test Plutono accessibility:**
```bash
curl http://localhost:3000
```

3. **Check collector logs:**
```bash
docker-compose logs collector
```

## Access Dashboard

### Initial Login

1. **Open your web browser** and navigate to:
```
http://localhost:3000
```

2. **Login with default credentials:**
   - Username: `admin`
   - Password: `admin`

3. **Change default password** when prompted (required for security)

### Dashboard Configuration

The application comes with a pre-configured dashboard named **"Cyclic Retentive Backup"** that includes:

- Real-time S7-1500V statistics
- Historical trend analysis
- Performance metrics visualization
- System health indicators

### S7-1500V Selection

If you have configured multiple S7-1500V instances:

1. **Use the dropdown menu** in the upper left corner of the dashboard
2. **Select the S7-1500V instance** you want to monitor
3. **The dashboard will automatically update** to show data from the selected instance

## Proxy Configuration

If your environment requires proxy server access for internet connectivity, configure both Docker and the compose file.

### Docker Daemon Configuration

Create or edit `/etc/docker/daemon.json`:

```json
{
    "proxies": {
        "http-proxy": "http://proxy.server.com:3128",
        "https-proxy": "http://proxy.server.com:3128",
        "no-proxy": "localhost,127.0.0.1"
    }
}
```

**Restart Docker daemon:**
```bash
sudo systemctl restart docker
```

### Compose Build Configuration

If proxy is required during build time, update the `compose.yaml` file:

```yaml
services:
  collector:
    build:
      context: ./src/vplc_collector
      args:
        http_proxy: http://proxy.server.com:3128
        https_proxy: http://proxy.server.com:3128
```

**Rebuild with proxy settings:**
```bash
docker-compose build --no-cache
docker-compose up -d
```

---

## Troubleshooting

**Common Issues:**

1. **Port conflicts:** Ensure port 3000 is available for Plutono dashboard
2. **Configuration errors:** Verify `cfg-data/secrets.json` format matches the example
3. **Network connectivity:** Check S7-1500V endpoint accessibility from Docker containers
4. **Proxy issues:** Verify proxy configuration if internet access is required during build

**Logs and Debugging:**
```bash
# View all service logs
docker-compose logs

# View specific service logs
docker-compose logs collector
docker-compose logs prometheus
docker-compose logs plutono

# Follow live logs
docker-compose logs -f collector
```