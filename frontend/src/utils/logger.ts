type LogLevel = 'DEBUG' | 'INFO' | 'WARN' | 'ERROR';

class Logger {
  private log(level: LogLevel, msg: string, meta?: Record<string, unknown>) {
    const timestamp = new Date().toISOString();
    const formatted = {
      time: timestamp,
      level,
      msg,
      ...meta,
    };

    switch (level) {
      case 'DEBUG':
        console.debug(JSON.stringify(formatted));
        break;
      case 'INFO':
        console.info(JSON.stringify(formatted));
        break;
      case 'WARN':
        console.warn(JSON.stringify(formatted));
        break;
      case 'ERROR':
        console.error(JSON.stringify(formatted));
        break;
    }
  }

  debug(msg: string, meta?: Record<string, unknown>) {
    this.log('DEBUG', msg, meta);
  }

  info(msg: string, meta?: Record<string, unknown>) {
    this.log('INFO', msg, meta);
  }

  warn(msg: string, meta?: Record<string, unknown>) {
    this.log('WARN', msg, meta);
  }

  error(msg: string, meta?: Record<string, unknown>) {
    this.log('ERROR', msg, meta);
  }
}

export const logger = new Logger();
