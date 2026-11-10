"""Export the README diagrams to light and dark PNGs: python docs/diagrams/export.py (needs Playwright + Chromium)."""
from pathlib import Path
from playwright.sync_api import sync_playwright

HERE = Path(__file__).resolve().parent
FONTS = HERE.parents[1] / 'web' / 'static' / 'fonts'
# The HTML sources must be self-contained (no url()), so the panel fonts are injected at export time.
FONT_CSS = ''.join(
    f"@font-face {{ font-family: '{name}'; src: url('{(FONTS / file).as_uri()}') format('woff2'); font-weight: 400 700; }}"
    for name, file in (('Hanken Grotesk', 'hanken-grotesk-latin.woff2'), ('Sometype Mono', 'sometype-mono-latin.woff2'))
)

with sync_playwright() as p:
    browser = p.chromium.launch()
    for src in sorted(HERE.glob('*.html')):
        for theme in ('light', 'dark'):
            page = browser.new_page(viewport=dict(width=1100, height=900), device_scale_factor=2)
            page.goto(src.as_uri())
            page.add_style_tag(content=FONT_CSS)
            page.evaluate('t => document.documentElement.dataset.theme = t', theme)
            page.evaluate('() => document.fonts.load("600 12px Hanken Grotesk").then(() => document.fonts.load("10px Sometype Mono")).then(() => document.fonts.ready)')
            out = HERE / f'{src.stem}-{theme}.png'
            page.locator('svg').screenshot(path=str(out))
            page.close()
            print(out.name)
    browser.close()
