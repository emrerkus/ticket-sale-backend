import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// API varsayilan olarak http://localhost:8080. Frontend :5173'te calisir.
// CORS'u Go tarafinda cors middleware'i hallediyor (CORS_ORIGIN=http://localhost:5173).
export default defineConfig({
  plugins: [react()],
  server: { port: 5173 },
});
