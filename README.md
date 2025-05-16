# Tonitor - Beat Scalpers at Target.com

Tonitor is a powerful monitoring tool designed to help Pokémon TCG collectors compete with scalpers when purchasing limited-edition and high-demand Pokémon cards from Target.com.

## Why Tonitor?

Scalpers use bots and automation tools to purchase high-demand Pokémon products the moment they become available, making it nearly impossible for genuine collectors to get them at retail prices. Tonitor levels the playing field by:

- **Real-time stock monitoring**: Continuously checks Target.com inventory for specific Pokémon products
- **Anti-bot evasion**: Uses sophisticated browser fingerprinting techniques to avoid detection
- **Proxy support**: Distributes requests across multiple IP addresses to prevent rate limiting
- **Instant notifications**: Alerts you the moment your desired Pokémon products come in stock

## Features

- Monitors Target.com inventory using product TCINs (Target's product IDs)
- Automatically rotates between browser fingerprints to avoid bot detection
- Supports proxy rotation to prevent IP bans and rate limits
- Auto-adjusts request rates based on Target's server response
- Outputs stock status as JSON for easy integration with notification systems

## Usage

```bash
./tonitor <TCIN> [delay_ms] [proxy_list]
```

- `TCIN`: Target's product ID for the Pokémon card product you want to monitor
- `delay_ms`: Delay between checks in milliseconds (default: 30000)
- `proxy_list`: Comma-separated list of proxy URLs (optional)

### Example

```bash
# Monitor Pokémon TCG Elite Trainer Box with TCIN 83449367
./tonitor 83449367 15000 http://user:pass@proxy1.example.com,http://user:pass@proxy2.example.com
```

## Tips for Success

- Find the TCIN for Pokémon products by viewing the product page URL on Target.com (A-XXXXXXXX)
- Use multiple proxies to avoid rate limiting
- Set up notifications by piping the output to a Discord/Telegram bot
- Be ready to purchase immediately when an in-stock notification appears

## Responsible Use

Please use this tool responsibly. Tonitor is designed to help collectors compete with scalpers, not to deplete inventory unfairly. Only monitor products you genuinely intend to purchase.

## Legal Disclaimer

This software is provided for educational purposes only. Users are responsible for complying with Target's terms of service and all applicable laws when using this software.