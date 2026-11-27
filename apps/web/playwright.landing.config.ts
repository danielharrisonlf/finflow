import { defineConfig, devices } from "@playwright/test";
import base from "./playwright.config";

// Exercise the same landing contracts in Android Chromium and iOS WebKit.
export default defineConfig(base, {
  testMatch: "landing-themes.spec.ts",
  projects: [
    { name: "android-chromium", use: { ...devices["Pixel 7"] } },
    { name: "ios-webkit", use: { ...devices["iPhone 13"] } },
  ],
  outputDir: "./test-results/landing-browsers",
});
