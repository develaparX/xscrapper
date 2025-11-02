#!/bin/bash

# GMGN Telegram Bot - Docker Compose Helper Script

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${BLUE}================================${NC}"
    echo -e "${BLUE}  GMGN Telegram Bot Manager${NC}"
    echo -e "${BLUE}================================${NC}"
}

# Check if .env file exists
check_env() {
    if [ ! -f ".env" ]; then
        print_error ".env file not found!"
        print_status "Creating .env from template..."
        cp .env.example .env
        print_warning "Please edit .env file with your configuration before running the bot"
        exit 1
    fi
}

# Show usage
show_usage() {
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  start           Start Twitter monitoring only"
    echo "  start-all       Start both Twitter and Wallet monitoring"
    echo "  stop            Stop all services"
    echo "  restart         Restart all services"
    echo "  logs            Show logs for all services"
    echo "  logs-twitter    Show logs for Twitter service only"
    echo "  logs-wallet     Show logs for Wallet service only"
    echo "  build           Build Docker images"
    echo "  status          Show container status"
    echo "  clean           Stop and remove containers, networks, and images"
    echo "  update          Pull latest code and rebuild"
    echo "  help            Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 start                 # Start Twitter monitoring"
    echo "  $0 start-all             # Start both Twitter and Wallet monitoring"
    echo "  $0 logs -f               # Follow logs in real-time"
}

# Main script
case "${1:-help}" in
    "start")
        print_header
        check_env
        print_status "Starting Twitter monitoring..."
        docker compose up -d
        print_status "Twitter monitoring started successfully!"
        print_status "Use '$0 logs' to view logs"
        ;;
    
    "start-all")
        print_header
        check_env
        print_status "Starting Twitter and Wallet monitoring..."
        docker compose --profile wallets up -d
        print_status "Both services started successfully!"
        print_status "Use '$0 logs' to view logs"
        ;;
    
    "stop")
        print_header
        print_status "Stopping all services..."
        docker compose --profile wallets down
        print_status "All services stopped successfully!"
        ;;
    
    "restart")
        print_header
        print_status "Restarting all services..."
        docker compose --profile wallets down
        docker compose --profile wallets up -d
        print_status "All services restarted successfully!"
        ;;
    
    "logs")
        shift
        print_header
        print_status "Showing logs... (Press Ctrl+C to exit)"
        docker compose logs "$@"
        ;;
    
    "logs-twitter")
        shift
        print_header
        print_status "Showing Twitter service logs... (Press Ctrl+C to exit)"
        docker compose logs gmgn-scraper "$@"
        ;;
    
    "logs-wallet")
        shift
        print_header
        print_status "Showing Wallet service logs... (Press Ctrl+C to exit)"
        docker compose logs gmgn-wallet-scraper "$@"
        ;;
    
    "build")
        print_header
        print_status "Building Docker images..."
        docker compose build --no-cache
        print_status "Docker images built successfully!"
        ;;
    
    "status")
        print_header
        print_status "Container status:"
        docker compose ps
        ;;
    
    "clean")
        print_header
        print_warning "This will remove all containers, networks, and images!"
        read -p "Are you sure? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            print_status "Cleaning up..."
            docker compose --profile wallets down --rmi all --volumes --remove-orphans
            print_status "Cleanup completed!"
        else
            print_status "Cleanup cancelled."
        fi
        ;;
    
    "update")
        print_header
        print_status "Updating application..."
        git pull
        docker compose build --no-cache
        docker compose --profile wallets down
        docker compose --profile wallets up -d
        print_status "Update completed successfully!"
        ;;
    
    "help"|*)
        print_header
        show_usage
        ;;
esac