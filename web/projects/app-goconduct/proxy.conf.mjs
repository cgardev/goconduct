/**
 * Development-server proxy for the Connect services.
 *
 * The console calls its own origin, and the development server forwards every
 * service path to the `goconduct` server. The target follows the address the
 * development script chooses, so one configuration serves any local port.
 */
const target = process.env['GOCONDUCT_API'] ?? 'http://127.0.0.1:6062';

export default {
  '/goconduct.v1.GraphService': { target, secure: false, changeOrigin: false },
  '/goconduct.v1.QualityService': { target, secure: false, changeOrigin: false },
};
