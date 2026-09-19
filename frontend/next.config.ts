import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Fully static: every page prerenders and the API is called from the
  // browser, so Netlify serves plain files with no server runtime.
  output: "export",
};

export default nextConfig;
