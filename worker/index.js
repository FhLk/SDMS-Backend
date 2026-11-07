import { DurableObject } from "cloudflare:workers";

const API_PORT = 8081;
const INACTIVITY_TIMEOUT_MS = 10 * 60 * 1000;

/**
 * One named Durable Object owns the API container. Application state remains
 * in PostgreSQL and R2, so the container filesystem is intentionally ephemeral.
 */
export class SDMSContainer extends DurableObject {
	starting;

	constructor(ctx, env) {
		super(ctx, env);
		const container = ctx.container;
		if (container.running) {
			void ctx.blockConcurrencyWhile(() =>
				container.setInactivityTimeout(INACTIVITY_TIMEOUT_MS),
			);
		}
	}

	async fetch(request) {
		this.starting ??= this.startAndWaitForPort().finally(() => {
			this.starting = undefined;
		});
		await this.starting;

		const url = new URL(request.url);
		url.protocol = "http:";
		url.host = "container";
		const forwarded = new Request(url, request);
		forwarded.headers.delete("host");

		return this.ctx.container.getTcpPort(API_PORT).fetch(forwarded);
	}

	async startAndWaitForPort() {
		const container = this.ctx.container;
		if (!container.running) {
			container.start({
				image: container.images.base,
				instance: "lite",
				enableInternet: true,
				env: {
					SERVER_HOST: "0.0.0.0",
					SERVER_PORT: String(API_PORT),
					DATABASE_URL: this.env.DATABASE_URL,
					DB_SCHEMA: "sdms",
					DB_CONNECT_TIMEOUT: "10s",
					DB_MAX_IDLE_CONNS: "2",
					DB_MAX_OPEN_CONNS: "10",
					DB_CONN_MAX_LIFETIME: "1h",
					DB_CONN_MAX_IDLE_TIME: "5m",
					SESSION_TTL: "24h",
					SESSION_COOKIE_NAME: "sdms_session",
					SESSION_COOKIE_SECURE: "true",
					FILE_STORAGE_DRIVER: "r2",
					R2_ACCOUNT_ID: this.env.R2_ACCOUNT_ID,
					R2_ACCESS_KEY_ID: this.env.R2_ACCESS_KEY_ID,
					R2_SECRET_ACCESS_KEY: this.env.R2_SECRET_ACCESS_KEY,
					R2_BUCKET: this.env.R2_BUCKET,
					R2_PREFIX: "sdms",
				},
			});
		}

		await container.setInactivityTimeout(INACTIVITY_TIMEOUT_MS);
		const port = container.getTcpPort(API_PORT);
		let lastError;

		for (let attempt = 0; attempt < 100; attempt++) {
			try {
				const response = await port.fetch("http://container/api/health", {
					signal: AbortSignal.timeout(1000),
				});
				await response.body?.cancel();
				if (!response.ok) {
					throw new Error(`Health check returned ${response.status}`);
				}
				return;
			} catch (error) {
				lastError = error;
				await scheduler.wait(200);
			}
		}

		throw new Error("SDMS API did not become ready on port 8081", {
			cause: lastError,
		});
	}
}

export default {
	fetch(request, env) {
		return env.SDMS_CONTAINER.getByName("primary").fetch(request);
	},
};
