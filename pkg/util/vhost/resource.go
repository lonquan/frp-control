// Copyright 2017 fatedier, fatedier@gmail.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vhost

import (
	"bytes"
	"io"
	"net/http"
	"os"

	"github.com/fatedier/frp/pkg/util/log"
	"github.com/fatedier/frp/pkg/util/version"
)

var NotFoundPagePath = ""

const (
	NotFound = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="light">
<title>404 · 暂未找到可用的 frp 代理</title>
<style>
    :root {
        color-scheme: light;
        --page-bg: #f3f6fb;
        --surface: #ffffff;
        --text: #172033;
        --text-muted: #55647b;
        --brand: #477ee5;
        --brand-ink: #1d4ed8;
        --brand-soft: #edf3ff;
        --shadow: 0 24px 70px rgb(23 32 51 / 10%);
    }

    *,
    *::before,
    *::after {
        box-sizing: border-box;
    }

    html {
        min-height: 100%;
        background: var(--page-bg);
    }

    body {
        min-height: 100vh;
        min-height: 100dvh;
        margin: 0;
        padding: 32px 20px;
        display: flex;
        color: var(--text);
        background:
            radial-gradient(circle at 12% 8%, rgb(71 126 229 / 16%), transparent 32rem),
            radial-gradient(circle at 88% 92%, rgb(14 116 144 / 10%), transparent 30rem),
            linear-gradient(145deg, #f8faff 0%, var(--page-bg) 48%, #eef4fb 100%);
        font-family:
            -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC",
            "Hiragino Sans GB", "Microsoft YaHei", sans-serif;
        font-size: 16px;
        line-height: 1.6;
        -webkit-font-smoothing: antialiased;
        text-rendering: optimizeLegibility;
    }

    body::before {
        position: fixed;
        inset: 0;
        content: "";
        pointer-events: none;
        background-image:
            linear-gradient(rgb(71 126 229 / 5%) 1px, transparent 1px),
            linear-gradient(90deg, rgb(71 126 229 / 5%) 1px, transparent 1px);
        background-size: 48px 48px;
        mask-image: linear-gradient(to bottom, #000 0%, transparent 72%);
    }

    .page {
        position: relative;
        width: min(100%, 600px);
        min-width: 0;
        margin: auto;
    }

    .brand {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 20px;
        margin-bottom: 16px;
        padding: 0 6px;
    }

    .brand-lockup {
        display: flex;
        min-width: 0;
        align-items: center;
        gap: 12px;
    }

    .brand-logo {
        width: 40px;
        height: 40px;
        flex: 0 0 auto;
        filter: drop-shadow(0 7px 12px rgb(71 126 229 / 20%));
    }

    .brand-name {
        margin: 0;
        color: var(--text);
        font-size: 1.0625rem;
        font-weight: 700;
        letter-spacing: -0.01em;
        line-height: 1.3;
    }

    .card {
        position: relative;
        overflow: hidden;
        border: 1px solid rgb(223 229 239 / 92%);
        border-radius: 24px;
        background: var(--surface);
        box-shadow: var(--shadow);
    }

    .card::before {
        position: absolute;
        inset: 0 0 auto;
        height: 4px;
        background: linear-gradient(90deg, var(--brand), #0e7490);
        content: "";
    }

    .card-content {
        padding: 34px 36px 32px;
    }

    .status {
        display: flex;
        width: fit-content;
        min-height: 30px;
        align-items: center;
        gap: 8px;
        margin-bottom: 22px;
        padding: 0 10px;
        border-radius: 8px;
        color: var(--brand-ink);
        background: var(--brand-soft);
        font-size: 0.75rem;
        font-weight: 700;
        letter-spacing: 0.03em;
    }

    .status::before {
        width: 6px;
        height: 6px;
        border-radius: 50%;
        background: currentColor;
        content: "";
    }

    h1 {
        max-width: 18ch;
        margin: 0;
        font-size: clamp(1.875rem, 6vw, 2.5rem);
        font-weight: 750;
        letter-spacing: -0.04em;
        line-height: 1.16;
        text-wrap: balance;
    }

    .title-line {
        display: inline;
    }

    .description {
        max-width: 50ch;
        margin: 14px 0 0;
        color: var(--text-muted);
        font-size: 1rem;
        line-height: 1.7;
    }

    code {
        padding: 0.12em 0.38em;
        border: 1px solid rgb(71 126 229 / 12%);
        border-radius: 5px;
        color: #263b62;
        background: #f3f7ff;
        font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
        font-size: 0.84em;
        overflow-wrap: anywhere;
    }

    .repo-link {
        display: flex;
        width: fit-content;
        min-height: 44px;
        align-items: center;
        justify-content: center;
        gap: 8px;
        margin-top: 24px;
        padding: 0 14px;
        border: 1px solid rgb(71 126 229 / 20%);
        border-radius: 8px;
        color: var(--brand-ink);
        background: var(--brand-soft);
        font-size: 0.875rem;
        font-weight: 700;
        text-decoration: none;
        text-underline-offset: 4px;
        transition:
            color 160ms ease,
            background-color 160ms ease;
    }

    .repo-link:hover {
        color: #173ea5;
        background: #e2ecff;
        text-decoration: underline;
    }

    .repo-link:active {
        color: #102c76;
    }

    .repo-link:focus-visible {
        outline: 3px solid rgb(29 78 216 / 34%);
        outline-offset: 3px;
    }

    .repo-link svg {
        width: 16px;
        height: 16px;
        flex: 0 0 auto;
    }

    @media (max-width: 560px) {
        body {
            padding: 22px 14px;
        }

        .brand {
            margin-bottom: 14px;
            padding: 0 4px;
        }

        .card {
            border-radius: 20px;
        }

        .card-content {
            padding: 28px 20px 24px;
        }

        h1 {
            font-size: clamp(1.75rem, 9vw, 2.2rem);
        }

        .title-line {
            display: block;
        }

        .description {
            margin-top: 12px;
            font-size: 0.9375rem;
        }

        .repo-link {
            width: 100%;
            padding: 0 10px;
            margin-top: 22px;
        }
    }

    @media (prefers-reduced-motion: reduce) {
        .repo-link {
            transition: none;
        }
    }
</style>
</head>
<body>
<main class="page" aria-labelledby="page-title">
    <header class="brand">
        <div class="brand-lockup">
            <svg class="brand-logo" viewBox="0 0 100 100" aria-hidden="true" focusable="false">
                <circle cx="50" cy="50" r="46" fill="#477EE5"/>
                <g transform="translate(50 50) skewX(-12) translate(-50 -50)">
                    <path d="M37 28 V72 M37 28 H63 M37 50 H55" fill="none" stroke="#FFFFFF"
                        stroke-width="14" stroke-linecap="round" stroke-linejoin="round"/>
                </g>
            </svg>
            <p class="brand-name">FRP-Control</p>
        </div>
    </header>

    <section class="card">
        <div class="card-content">
            <span class="status">404 · 未找到代理</span>
            <h1 id="page-title">暂未找到 <span class="title-line">可用的 frp 代理</span></h1>
            <p class="description">
                当前域名未匹配到 HTTP/HTTPS 代理。请在 frpc 中配置
                <code>customDomains</code>，或使用服务端已启用的
                <code>subdomain</code>，确认 DNS 指向当前 frps 后重载客户端。
            </p>

            <a class="repo-link"
                href="https://github.com/lonquan/frp-control"
                target="_blank"
                rel="noopener noreferrer"
                aria-label="访问 FRP-Control GitHub 仓库（在新窗口打开）">
                查看 GitHub 仓库
                <svg viewBox="0 0 20 20" fill="none" aria-hidden="true" focusable="false">
                    <path d="M7.5 4.5h8v8M15.5 4.5l-11 11" stroke="currentColor"
                        stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
            </a>
        </div>
    </section>
</main>
</body>
</html>
`
)

func getNotFoundPageContent() []byte {
	var (
		buf []byte
		err error
	)
	if NotFoundPagePath != "" {
		buf, err = os.ReadFile(NotFoundPagePath)
		if err != nil {
			log.Warnf("read custom 404 page error: %v", err)
			buf = []byte(NotFound)
		}
	} else {
		buf = []byte(NotFound)
	}
	return buf
}

func setNotFoundHeaders(header http.Header) {
	header.Set("server", "frp/"+version.Full())
	header.Set("Content-Type", "text/html; charset=utf-8")
}

func NotFoundResponse() *http.Response {
	header := make(http.Header)
	setNotFoundHeaders(header)

	content := getNotFoundPageContent()
	res := &http.Response{
		Status:        "Not Found",
		StatusCode:    404,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(content)),
		ContentLength: int64(len(content)),
	}
	return res
}
