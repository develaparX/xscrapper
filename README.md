# GMGN Twitter/X Tracker

A robust Twitter/X monitoring bot for GMGN.ai with automatic token management and browser automation.

## Features

- 🔐 **Automated Telegram Login** with browser automation
- 🔄 **Robust Token Management** with auto-refresh and rotation
- 🍪 **Cookie Management** for session persistence
- 🐦 **Real-time Twitter/X Tracking** with keyword filtering
- 📱 **Telegram Notifications** for new relevant tweets
- 🖥️ **VPS Compatible** with KasmVNC support

## Prerequisites

### System Requirements

- Ubuntu 24.04 LTS (VPS compatible)
- Go 1.19 or higher
- Thorium Browser (or Chromium)
- KasmVNC (for VPS GUI access)

### Required Accounts

- GMGN.ai account
- Telegram Bot Token
- Telegram Chat ID

## Installation on Ubuntu 24 VPS

### Step 1: Update System

```bash
sudo apt update && sudo apt upgrade -y
```

### Step 2: Install Go

```bash
# Download and install Go
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz

# Add Go to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verify installation
go version
```

### Step 3: Install Dependencies

```bash
# Install required packages
sudo apt install -y git curl wget unzip

# Install additional dependencies for browser automation
sudo apt install -y libnss3 libatk-bridge2.0-0 libdrm2 libxcomposite1 libxdamage1 libxrandr2 libgbm1 libxss1 libasound2
```

### Step 4: Install Thorium Browser (if not already installed)

```bash
# Download Thorium Browser
wget https://github.com/Alex313031/thorium/releases/download/M117.0.5938.157/thorium-browser_117.0.5938.157_amd64.deb

# Install Thorium
sudo dpkg -i thorium-browser_117.0.5938.157_amd64.deb
sudo apt-get install -f  # Fix any dependency issues

# Verify installation
thorium-browser --version
```

### Step 5: Setup KasmVNC (if not already installed)

```bash
# Install KasmVNC
wget https://github.com/kasmtech/KasmVNC/releases/download/v1.2.0/kasmvncserver_jammy_1.2.0_amd64.deb
sudo dpkg -i kasmvncserver_jammy_1.2.0_amd64.deb
sudo apt-get install -f

# Setup VNC password
vncpasswd

# Start KasmVNC (replace :1 with your preferred display)
vncserver :1 -geometry 1920x1080 -depth 24
```

### Step 6: Clone and Setup Project

```bash
# Clone the repository
git clone <your-repo-url>
cd go-xscrapper-tele

# Install Go dependencies
go mod tidy
```

### Step 7: Configure Environment Variables

```bash
# Copy example environment file
cp .env.example .env

# Edit environment file
nano .env
```

Add the following configuration to `.env`:

```env
# GMGN Authentication (will be auto-generated via Telegram login)
BEARER_TOKEN=
COOKIES=

# Telegram Bot Configuration
TELEGRAM_BOT_TOKEN=your_bot_token_here
TELEGRAM_CHAT_ID=your_chat_id_here

# GMGN Credentials (optional, for fallback)
GMGN_EMAIL=your_email@example.com
GMGN_PASSWORD=your_password_here
```

### Step 8: Get Telegram Bot Token and Chat ID

#### Create Telegram Bot:

1. Message [@BotFather](https://t.me/botfather) on Telegram
2. Send `/newbot`
3. Follow instructions to create your bot
4. Copy the bot token

#### Get Chat ID:

1. Start a chat with your bot
2. Send any message to your bot
3. Visit: `https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getUpdates`
4. Find your chat ID in the response

## Usage

### Method 1: Traditional Twitter Monitoring (Simple)

```bash
# Monitor Twitter with Telegram notifications every 10 seconds
go run . -api=twitter -telegram -refresh
```

### Method 2: Advanced Monitoring with Token Management

```bash
# Start robust monitoring with automatic token rotation
go run . -monitor
```

### Method 3: One-time Data Fetch

```bash
# Fetch Twitter data once
go run . -api=twitter

# Fetch wallet data once
go run . -api=wallets

# Send to Telegram
go run . -api=twitter -telegram
```

## First Time Setup (Authentication)

When you run the bot for the first time, it will prompt for Telegram login:

### Option 1: Automated Browser Login (Recommended for VPS)

1. Run the bot: `go run . -monitor`
2. When prompted, type: `auto`
3. The system will open Thorium browser automatically
4. Click the Telegram bot link that appears
5. Complete the Telegram login process
6. The system will automatically extract the token

### Option 2: Manual Login

1. Run the bot: `go run . -monitor`
2. Click the Telegram bot link manually in your browser
3. Complete the login process
4. Copy the response URL (format: `https://gmgn.ai/tglogin?user_id=...`)
5. Paste it when prompted

## VPS-Specific Instructions

### Accessing GUI on VPS via KasmVNC

1. **Connect to VNC:**

   ```bash
   # If KasmVNC is running on display :1
   # Access via web browser: http://your-vps-ip:6901
   # Or use VNC client: your-vps-ip:5901
   ```

2. **Set Display Environment:**

   ```bash
   export DISPLAY=:1
   ```

3. **Run with GUI:**
   ```bash
   # Make sure you're in the VNC session
   cd /path/to/go-xscrapper-tele
   go run . -monitor
   ```

### Running as Background Service

Create a systemd service for continuous monitoring:

```bash
# Create service file
sudo nano /etc/systemd/system/gmgn-tracker.service
```

Add the following content:

```ini
[Unit]
Description=GMGN Twitter Tracker
After=network.target

[Service]
Type=simple
User=your-username
WorkingDirectory=/path/to/go-xscrapper-tele
Environment=DISPLAY=:1
ExecStart=/usr/local/go/bin/go run . -monitor
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable gmgn-tracker
sudo systemctl start gmgn-tracker

# Check status
sudo systemctl status gmgn-tracker

# View logs
sudo journalctl -u gmgn-tracker -f
```

## Configuration Options

### Command Line Flags

- `-api=twitter`: Monitor Twitter/X messages
- `-api=wallets`: Monitor wallet data
- `-telegram`: Send notifications to Telegram
- `-refresh`: Auto-refresh every 10 seconds
- `-monitor`: Start advanced monitoring with token management
- `-output=file.json`: Save output to file

### Environment Variables

- `BEARER_TOKEN`: GMGN API bearer token (auto-generated)
- `COOKIES`: Session cookies (auto-managed)
- `TELEGRAM_BOT_TOKEN`: Your Telegram bot token
- `TELEGRAM_CHAT_ID`: Your Telegram chat ID
- `GMGN_EMAIL`: GMGN account email (optional)
- `GMGN_PASSWORD`: GMGN account password (optional)

## Troubleshooting

### Common Issues

1. **Browser not opening:**

   ```bash
   # Check if Thorium is installed
   thorium-browser --version

   # Check display
   echo $DISPLAY

   # Test browser manually
   thorium-browser --no-sandbox --disable-gpu
   ```

2. **VNC connection issues:**

   ```bash
   # Restart VNC server
   vncserver -kill :1
   vncserver :1 -geometry 1920x1080 -depth 24
   ```

3. **Token validation fails:**

   - The bot will automatically refresh tokens
   - Check Telegram notifications for refresh status
   - Manually restart if needed

4. **Permission issues:**
   ```bash
   # Fix permissions
   chmod +x run.sh
   sudo chown -R $USER:$USER .
   ```

### Logs and Debugging

- Bot logs are printed to console
- Use `sudo journalctl -u gmgn-tracker -f` for service logs
- Check Telegram for authentication notifications

## Security Notes

- Never share your `.env` file
- Keep your Telegram bot token secure
- The bot automatically manages authentication tokens
- Cookies and tokens are stored locally in `.env`

## Performance Tips

- Use `-monitor` mode for production (more efficient)
- Adjust refresh intervals based on your needs
- Monitor VPS resources (CPU/Memory usage)
- Use systemd service for automatic restarts

## Support

If you encounter issues:

1. Check the logs for error messages
2. Verify all environment variables are set
3. Ensure Thorium browser is properly installed
4. Test VNC connection manually

## License

[Your License Here]
