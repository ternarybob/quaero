# URL Fetching

When the user pastes a URL (http:// or https://), automatically fetch it using curl and analyze the response.

For localhost URLs, use: `curl -s <url>`

For external URLs, use: `curl -s -L <url>` (follow redirects)

If the response is HTML and appears to be a JS-rendered app (minimal body content, lots of script tags), note that the actual content may require a browser to render.