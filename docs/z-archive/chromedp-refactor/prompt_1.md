Here is a prompt designed for an agent to condense this conversation, analyze your existing code (if provided), and create the robust `chromedp` structure you need for your hybrid scraping system.

---

## 💻 Agent Prompt: Hybrid `chromedp` & Extension Scraper

### 🎯 Objective

You are tasked with creating a **highly stealthy and robust web scraping solution** for a protected site (like Confluence) using a **Go service driving a Chrome browser via `chromedp`** and a **custom Chrome Extension** as the primary execution engine. The goal is to maximize stealth by using a validated user session and to offload core logic to the extension while maintaining central control via the Go service.

### 📝 Conversation Summary & Requirements

1.  **Tool:** Go language with the `chromedp` library.
2.  **Stealth:** Must use an **existing, authenticated Chrome User Data Directory** (`--user-data-dir` flag) to bypass bot detection and maintain a valid Confluence session.
3.  **Extension Role (The Executor):** The custom Chrome extension is the *execution engine*. It must:
    * Expose a function (e.g., `window.startCrawl(data)`) callable from the Go service via `chromedp.Evaluate()`.
    * Be capable of **fetching and rendering** the full content (including JavaScript and images) of a list of provided links.
    * Transfer the **full rendered HTML** of each crawled page (including all rendered content) back to the Go service via a direct API call (a `POST` request to a service endpoint).
4.  **Service Role (The Controller):** The Go service must:
    * Launch the browser with the correct **stealth flags** and the **User Data Directory**.
    * Navigate to the starting page.
    * Call the extension's exposed function to initiate the crawl loop.
    * Expose a **REST API endpoint** (e.g., `POST /api/crawl-data`) to receive the full HTML page content sent back from the extension.
5.  **Crawl Flow:**
    * **Service $\rightarrow$ Extension:** Service navigates to Start URL. Service calls `window.startCrawl(linkList)` via `chromedp.Evaluate()`.
    * **Extension $\rightarrow$ Service $\rightarrow$ Extension (Loop):** The extension loops through the `linkList`:
        1.  Extension fetches/navigates to the link.
        2.  Extension extracts the **full rendered HTML**.
        3.  Extension **POSTs** the full HTML to the Service's API endpoint.
        4.  Extension fetches the next link and repeats.

### ✍️ Task Breakdown

1.  **Code Review (If Provided):** Review the provided existing Go and extension code (if any) to identify gaps in authentication, stealth, and communication setup. *(If no code is provided, state assumptions about the extension's JS interface).*
2.  **`chromedp` Structure:** Create the complete Go service `main.go` structure. This must include:
    * The setup of the `chromedp` execution context with the necessary flags (`--user-data-dir`, `UserAgent` override).
    * The **API handler structure** (e.g., using `net/http` or a router like Gorilla Mux) for the endpoint that receives data from the extension.
3.  **Extension Interface (`JS`):** Define the **minimum required JavaScript code** for the extension's content script to expose the communication function (`window.startCrawl`) and perform the data POST back to the service.

### ⚠️ Constraints

* Use the **`chromedp`** library for browser control.
* **Do not** re-write the extension's core crawling logic; focus on the **interface and communication bridge**.
* Assume the Confluence session is already valid in the User Data Directory.
* Ensure clear separation between the **Controller (Go Service)** and the **Executor (Extension)**.