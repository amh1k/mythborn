const demoAuth = new URL('./demo-auth.tsx', import.meta.url).pathname

export default {
  root: '/home/amh1k/projects/mythborn/web',
  plugins: [
    {
      name: 'mythborn-local-demo-auth',
      enforce: 'pre',
      resolveId(source, importer) {
        if ((source === './auth' || source === '../auth') && importer?.includes('/web/src/')) return demoAuth
      },
    },
  ],
  server: { host: '127.0.0.1', port: 5187, strictPort: true },
}
