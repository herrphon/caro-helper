import path from "node:path"
import { defineConfig } from "vite"
import react from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
  },
  build: {
    // the Go binary embeds this directory
    outDir: "../cmd/carohelper/dist",
    emptyOutDir: true,
  },
  server: {
    // during `npm run dev`, forward API calls to the Go server
    proxy: { "/api": "http://localhost:8765" },
  },
})
