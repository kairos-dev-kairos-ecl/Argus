/**
 * Express middleware for automatic instrumentation
 */

import { Request, Response, NextFunction } from 'express';
import { ArgusClient } from './client';
import { Layer, Severity } from './types';

export interface ArgusMiddlewareOptions {
  client?: ArgusClient;
  layer?: Layer;
  category?: string;
  severity?: Severity;
  excludePaths?: string[];
  includeRequestBody?: boolean;
  includeResponseBody?: boolean;
}

/**
 * Express middleware for automatic signal emission
 */
export function argusMiddleware(options: ArgusMiddlewareOptions = {}) {
  const {
    client = null,
    layer = Layer.L9_API_GATEWAY,
    category = 'http.request',
    severity = Severity.INFO,
    excludePaths = ['/health', '/metrics'],
    includeRequestBody = false,
    includeResponseBody = false,
  } = options;

  return (req: Request, res: Response, next: NextFunction) => {
    // Skip excluded paths
    if (excludePaths.some((path) => req.path === path || req.path.startsWith(path + '/'))) {
      return next();
    }

    const startTime = Date.now();
    const traceId = req.headers['x-trace-id'] as string || undefined;

    // Wrap response.send to capture response
    const originalSend = res.send.bind(res);
    res.send = function (data: any) {
      const durationMs = Date.now() - startTime;

      // Emit signal if client available (fail-open)
      if (client) {
        const context: any = {
          method: req.method,
          path: req.path,
          status: res.statusCode,
        };

        if (includeRequestBody && req.body) {
          context.request_body = JSON.stringify(req.body).substring(0, 1000);
        }

        if (includeResponseBody && data) {
          const bodyStr = typeof data === 'string' ? data : JSON.stringify(data);
          context.response_body = bodyStr.substring(0, 1000);
        }

        client
          .emitSignal(
            layer,
            category,
            severity,
            context,
            durationMs,
            traceId,
          )
          .catch((err) => console.error('Failed to emit signal:', err));
      }

      return originalSend(data);
    };

    next();
  };
}

/**
 * Decorator for automatic signal emission on Express route handlers
 */
export function observeRoute(
  layer: Layer = Layer.L9_API_GATEWAY,
  category: string = 'http.request',
  client: ArgusClient | null = null,
) {
  return (target: any, propertyKey: string, descriptor: PropertyDescriptor) => {
    const originalMethod = descriptor.value;

    descriptor.value = async function (req: Request, res: Response, next: NextFunction) {
      const startTime = Date.now();
      const traceId = req.headers['x-trace-id'] as string || undefined;

      try {
        const result = await originalMethod.call(this, req, res, next);

        const durationMs = Date.now() - startTime;

        if (client) {
          await client.emitSignal(
            layer,
            category,
            Severity.INFO,
            { status: res.statusCode },
            durationMs,
            traceId,
          );
        }

        return result;
      } catch (error) {
        const durationMs = Date.now() - startTime;

        if (client) {
          await client.emitSignal(
            layer,
            category,
            Severity.HIGH,
            { error: String(error), status: res.statusCode || 500 },
            durationMs,
            traceId,
          );
        }

        throw error;
      }
    };

    return descriptor;
  };
}
