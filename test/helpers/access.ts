import { exportJWK, generateKeyPair, type JWK, SignJWT } from "jose";

export interface TestKey {
  kid: string;
  privateKey: CryptoKey;
  publicJwk: JWK;
}

export async function createTestKey(kid: string): Promise<TestKey> {
  const { publicKey, privateKey } = await generateKeyPair("RS256", { extractable: true });
  const publicJwk = await exportJWK(publicKey);
  return { kid, privateKey, publicJwk: { ...publicJwk, kid, alg: "RS256", use: "sig" } };
}

export interface SignOptions {
  teamDomain?: string;
  aud?: string;
  email?: string;
  /** 発行時刻(エポック秒)。 */
  iat?: number;
  /** 期限(エポック秒)。null なら exp クレームを付けない(必須クレーム欠落の検証用)。 */
  exp?: number | null;
  /**
   * service token(CF-Access-Client-Id / Secret)で認証したときの形にする。値は common_name(Client ID)。
   * Cloudflare の Application token の仕様どおり、email を持たず sub は空文字になる。
   */
  serviceClientId?: string;
  /** 人の JWT から email を外す(identity のない JWT の拒否を確かめる用)。 */
  omitEmail?: boolean;
}

export async function signAccessJwt(key: TestKey, opts: SignOptions = {}): Promise<string> {
  const iat = opts.iat ?? Math.floor(Date.now() / 1000);
  const service = opts.serviceClientId !== undefined;
  const claims: Record<string, unknown> = { type: "app" };
  if (service) claims.common_name = opts.serviceClientId;
  else if (!opts.omitEmail) claims.email = opts.email ?? "alice@example.com";
  const jwt = new SignJWT(claims)
    .setProtectedHeader({ alg: "RS256", kid: key.kid })
    .setIssuer(`https://${opts.teamDomain ?? "board-test.cloudflareaccess.com"}`)
    .setAudience(opts.aud ?? "test-access-aud")
    .setSubject(service ? "" : "access-user-1")
    .setIssuedAt(iat);
  if (opts.exp !== null) jwt.setExpirationTime(opts.exp ?? iat + 300);
  return jwt.sign(key.privateKey);
}

export function jwksResponse(...keys: TestKey[]): Response {
  return new Response(JSON.stringify({ keys: keys.map((k) => k.publicJwk) }), {
    headers: { "Content-Type": "application/json" },
  });
}
