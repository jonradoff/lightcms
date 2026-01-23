#!/bin/bash

# Migrate theme settings script
# Usage: ADMIN_PASSWORD=yourpassword ./migrate_theme.sh

if [ -z "$ADMIN_PASSWORD" ]; then
    echo "Error: ADMIN_PASSWORD environment variable is required"
    echo "Usage: ADMIN_PASSWORD=yourpassword ./migrate_theme.sh"
    exit 1
fi

# Get cookies first
curl -c /tmp/cookies.txt -b /tmp/cookies.txt -X POST -d "password=$ADMIN_PASSWORD" -s -o /dev/null http://localhost:8082/cm/login

# Update theme with URL-encoded data
curl -b /tmp/cookies.txt -X POST \
  --data-urlencode "site_name=Metavert" \
  --data-urlencode "site_tagline=Building the Metaverse" \
  --data-urlencode "primary_color=#6366f1" \
  --data-urlencode "secondary_color=#8b5cf6" \
  --data-urlencode "accent_color=#06b6d4" \
  --data-urlencode "background_color=#0f172a" \
  --data-urlencode "text_color=#f1f5f9" \
  --data-urlencode "font_family=Inter, system-ui, sans-serif" \
  --data-urlencode "heading_font=Space Grotesk, system-ui, sans-serif" \
  --data-urlencode "border_radius=12px" \
  --data-urlencode 'header_html=<nav class="main-nav">
    <div class="nav-container">
        <a href="/" class="logo">Metavert</a>
        <div class="nav-links">
            <a href="https://metavert.substack.com" target="_blank">Substack</a>
            <a href="/metaverttv">Metavert.TV</a>
            <a href="/projects">Projects</a>
            <a href="/contact">Contact</a>
        </div>
    </div>
</nav>' \
  --data-urlencode 'footer_html=<div class="footer-grid">
    <div class="footer-section">
        <h4>Media</h4>
        <ul>
            <li><a href="https://metavert.substack.com" target="_blank">Metavert Meditations</a></li>
            <li><a href="https://youtube.com/@metavert" target="_blank">YouTube</a></li>
            <li><a href="https://anchor.fm/buildingthemetaverse" target="_blank">Podcast</a></li>
        </ul>
    </div>
    <div class="footer-section">
        <h4>Socials</h4>
        <ul>
            <li><a href="https://linkedin.com/in/jonradoff" target="_blank">LinkedIn</a></li>
            <li><a href="https://twitter.com/jradoff" target="_blank">Twitter</a></li>
            <li><a href="https://tiktok.com/@metavert" target="_blank">TikTok</a></li>
            <li><a href="https://discord.gg/metavert" target="_blank">Discord</a></li>
        </ul>
    </div>
    <div class="footer-section">
        <h4>Legal</h4>
        <ul>
            <li><a href="/privacy-policy">Privacy Policy</a></li>
            <li><a href="/terms-of-service">Terms of Service</a></li>
        </ul>
    </div>
</div>
<p style="text-align:center;margin-top:2rem;">Content licensed under <a href="https://creativecommons.org/licenses/by/4.0/" target="_blank">Creative Commons Attribution 4.0</a></p>
<p style="text-align:center;">© 2026 <a href="https://metavert.io" target="_blank">Metavert LLC</a></p>' \
  -s -w "%{http_code}" http://localhost:8082/cm/theme
