# GMGN Telegram Bot

Real-time monitoring bot untuk GMGN.ai yang mengirim notifikasi Twitter messages dan wallet data langsung ke Telegram.

## 🚀 Features

- ✅ Real-time monitoring Twitter messages dari GMGN.ai
- ✅ Real-time monitoring wallet data
- ✅ Telegram bot integration
- ✅ Duplicate prevention (tidak spam pesan lama)
- ✅ Auto refresh setiap 10 detik
- ✅ Docker support dengan Docker Compose
- ✅ Memory management dengan auto cleanup
- ✅ Health checks dan restart otomatis

## 📋 Prerequisites

1. **Docker & Docker Compose** terinstall
2. **Telegram Bot Token** dari @BotFather
3. **GMGN.ai Account** dengan bearer token dan cookies

## 🛠️ Setup

### 1. Clone Repository

```bash
git clone <repository-url>
cd gmgn-telegram-bot
```

### 2. Setup Environment Variables

```bash
# Copy template environment file
cp .env.example .env

# Edit .env file dengan konfigurasi Anda
nano .env
```

### 3. Dapatkan GMGN Bearer Token & Cookies

1. Login ke https://gmgn.ai
2. Buka Developer Tools (F12)
3. Pergi ke Network tab
4. Refresh halaman atau lakukan request
5. Cari request ke API gmgn.ai
6. Copy `Authorization: Bearer ...` dan `Cookie: ...` headers

### 4. Setup Telegram Bot

1. Chat dengan @BotFather di Telegram
2. Ketik `/newbot` dan ikuti instruksi
3. Dapatkan bot token
4. Tambahkan bot ke grup/channel atau chat pribadi
5. Dapatkan chat ID:
   - Kirim pesan ke bot
   - Buka: `https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getUpdates`
   - Cari `"chat":{"id":-1001234567890}` di response

## 🐳 Running with Docker Compose

### Easy Way (Using Helper Script)

```bash
# Make script executable
chmod +x run.sh

# Start Twitter monitoring only
./run.sh start

# Start both Twitter and Wallet monitoring
./run.sh start-all

# View logs in real-time
./run.sh logs -f

# View Twitter logs only
./run.sh logs-twitter -f

# Stop all services
./run.sh stop

# Check status
./run.sh status

# See all available commands
./run.sh help
```

### Manual Docker Compose Commands

```bash
# Twitter monitoring only
docker compose up -d

# Twitter + Wallet monitoring
docker compose --profile wallets up -d

# View logs
docker compose logs -f gmgn-scraper

# Stop services
docker compose down

# Build images
docker compose build

# Restart services
docker compose restart

# Update and restart
docker compose pull && docker compose up -d
```

## 🔧 Configuration Options

### Environment Variables

```env
# GMGN API
GMGN_BEARER_TOKEN=your_token_here
GMGN_COOKIES=your_cookies_here
GMGN_DEVICE_ID=7754041a-7df7-4454-80aa-2160aec01882
GMGN_CHAIN=bsc

# Telegram
TELEGRAM_BOT_TOKEN=1234567890:ABC...
TELEGRAM_CHAT_ID=-1001234567890
TELEGRAM_WALLET_CHAT_ID=-1001234567891  # Optional: separate chat for wallets
```

### Service Profiles

- **Default**: Hanya Twitter monitoring
- **wallets**: Twitter + Wallet monitoring

## 📱 Manual Usage (Without Docker)

### Prerequisites

```bash
# Install Go 1.25+
go version

# Install dependencies
go mod download
```

### Commands

```bash
# Twitter monitoring (realtime)
go run . -api=twitter -telegram -refresh

# Wallet monitoring (realtime)
go run . -api=wallets -telegram -refresh

# Single check (no realtime)
go run . -api=twitter -telegram
go run . -api=wallets -telegram

# Save to file + send to Telegram
go run . -api=twitter -telegram -output=data.json

# Console output only (no Telegram)
go run . -api=twitter
```

## 📊 Monitoring & Logs

### Docker Logs

```bash
# View real-time logs
docker compose logs -f

# View specific service logs
docker compose logs -f gmgn-scraper
docker compose logs -f gmgn-wallet-scraper

# View last 100 lines
docker compose logs --tail=100 gmgn-scraper
```

### Health Checks

Services include health checks yang akan restart container jika aplikasi crash.

## 🔄 Updates

### Update Application

```bash
# Pull latest changes
git pull

# Rebuild and restart
docker compose down
docker compose build --no-cache
docker compose up -d
```

### Update Dependencies

```bash
# Update Go modules
go mod tidy
go mod download

# Rebuild Docker images
docker compose build --no-cache
```

## 🐛 Troubleshooting

### Common Issues

1. **401 Unauthorized Error**

   - Bearer token expired → Update GMGN_BEARER_TOKEN
   - Cookies expired → Update GMGN_COOKIES

2. **Telegram Bot Error**

   - Invalid token → Check TELEGRAM_BOT_TOKEN
   - Can't send message → Check TELEGRAM_CHAT_ID
   - Bot not in group → Add bot to Telegram group/channel

3. **Container Keeps Restarting**
   - Check logs: `docker compose logs gmgn-scraper`
   - Verify environment variables
   - Check network connectivity

### Debug Commands

```bash
# Check container status
docker compose ps

# Enter container for debugging
docker compose exec gmgn-scraper sh

# Check environment variables
docker compose exec gmgn-scraper env

# Test single run
docker compose run --rm gmgn-scraper -api=twitter -telegram
```

## 📝 API Endpoints

- **Twitter Messages**: `https://gmgn.ai/vas/api/v1/twitter/messages`
- **Following Wallets**: `https://gmgn.ai/api/v1/follow/following_wallets_v2`

## 🔒 Security Notes

- Jangan commit file `.env` ke repository
- Rotate bearer token dan cookies secara berkala
- Gunakan environment variables untuk production
- Limit akses ke Telegram bot token

## 📄 License

MIT License - see LICENSE file for details.
