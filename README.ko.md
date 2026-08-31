<p align="center">
  <img src="docs/header.png" alt="termagitchi" width="100%" />
</p>

<p align="center">
  <strong>에이전트마다 한 마리씩, git worktree마다 하나의 둥지(den).</strong>
</p>

<p align="center">
  <a href="README.md">English</a> ·
  <a href="README.ja.md">日本語</a> ·
  <b>한국어</b> ·
  <a href="README.zh-Hans.md">简体中文</a>
</p>

<p align="center">
  흔한 터미널 펫은 <i>당신</i>의 것입니다. 한 마리를 오래 키우죠. 이건 다릅니다.
  여기서 펫은 <b>에이전트</b>의 것입니다. worktree는 각자 이름을 가진 둥지가 되고,
  그 안에서 돌아가는 에이전트마다 자기 생물을 하나씩 갖습니다. 기분은 그 worktree가
  얼마나 깔끔한지를 따라갑니다. 에이전트 여섯 개를 돌리면 생물도 여섯 마리가 동시에 살아 있고,
  한눈에 구분되기 때문에 지금 보고 있는 세션이 무엇인지, 어느 쪽이 문제인지 바로 알 수 있습니다.
</p>

<p align="center">
  <img src="docs/demo.gif" alt="worktree 여섯 개, 생물 여섯 마리" width="100%" />
</p>

## 설치

**Linux와 macOS**

```sh
curl -fsSL https://raw.githubusercontent.com/TevvvB/termagitchi/main/install.sh | sh
pets install
```

**macOS (Homebrew)**

```sh
brew install TevvvB/tap/termagitchi
pets install
```

**Windows** — `scoop bucket add tevvvb https://github.com/TevvvB/scoop-bucket` 후
`scoop install termagitchi`, 그리고 `pets install`을 실행하세요.
또는 [Releases](https://github.com/TevvvB/termagitchi/releases)에서 아카이브를 받아
`pets`를 직접 `PATH`에 두어도 됩니다.

**Go로 설치** — `go install github.com/TevvvB/termagitchi/cmd/pets@latest`

`pets install`은 설치된 에이전트를 찾아 스스로를 연결합니다. 건드린 설정은 백업하고
나머지 설정은 그대로 둡니다. 실행한 뒤에는 새 세션을 여세요. `pets uninstall`로 되돌립니다.

에이전트를 설치만 하고 한 번도 실행하지 않았다면 설정 디렉터리가 아직 없으므로
`pets install --harness=claude`처럼 직접 지정하세요.

## 읽는 법

| 기호 | 의미 |
|---|---|
| `@ DXB` | 둥지(den). 어느 worktree인지를 공항 코드로 |
| `•ᴗ•` `•_•` `¬_¬` `>_<` `@_@` `x_x` | 기분. 하트 5부터 0까지 |
| `7△` | 커밋되지 않은 파일 7개 |
| `3↑` | 푸시되지 않은 커밋 3개 |
| `105↓` | trunk보다 뒤처진 커밋 수. 40을 넘을 때만 표시 |
| `⑂2` | 마이그레이션 head가 2개 |
| `✗` | 마지막 테스트 또는 lint 실패 |
| `-.-` 와 `·····` | 세션 시작 직후, 아직 깨어나는 중 |

하트는 5에서 시작해 커밋되지 않은 파일이 있으면 1, 15개를 넘으면 하나 더,
푸시되지 않은 커밋이 있으면 1, 5개를 넘으면 하나 더, 테스트가 깨져 있으면 2가 깎입니다.
전부 설정으로 바꿀 수 있습니다.

테스트 결과는 러너가 결과를 명확히 말했을 때만 기록되고, 두 시간이 지난 결과는 잊힙니다.
이미 고친 브랜치에 빨간 표시가 계속 남지 않습니다.

## 모든 worktree를 한 번에 보기

```
  pets party                              4 dens · 3 agents

  @PAR (•_•)     otter ✦     refactor/auth-guard  ♥♥♥♥♡  3△
       (•_•)     otter     fix the flaky auth test               just now
       /•_•\     cat       audit the session middleware          just now
  @SYD -[•_•]-   carp  ✦     spike/wasm-build     ♥♥♥♥♡  9△
       -[•_•]-   carp      port the wasm build to esbuild        just now
  @MEX {•ᴗ•}     fox   ✦     chore/bump-deps      ♥♥♥♥♥
  @IST \(•ᴗ•)/   crow  ✦✦✦   docs/api-reference   ♥♥♥♥♥

  worst: otter · uncommitted
```

상태가 나쁜 것이 위로 오기 때문에 손봐야 할 것이 가장 눈에 띕니다. 에이전트가 있는 둥지에는
세션 이름과 함께 누가 있는지 나열되므로, 같은 worktree를 쓰는 에이전트 둘은 한 마리가 아니라
두 마리로 보입니다. 비어 있는 둥지에도 생물은 있습니다. 목록이 길면 잘리고,
`pets party --all`로 전부 볼 수 있습니다.

## 모으기

가챠입니다. 생물은 세션마다 부화하므로, worktree를 새로 여는 것이 곧 한 번의 뽑기입니다.
등급은 다섯 단계입니다.

| 등급 | 확률 | |
|---|---|---|
| common | 60% | `✦` |
| uncommon | 25% | `✦✦` |
| rare | 11% | `✦✦✦` |
| legendary | 3.5% | `✦✦✦✦` |
| mythic | 0.5% | `✦✦✦✦✦` |

rare 이상은 등급 고유의 색을 입기 때문에 별을 세지 않아도 무엇을 뽑았는지 보입니다.
common과 uncommon에는 각자의 색조가 배정되어 같은 종이라도 구분됩니다.

이로치(shiny)는 1/128의 별도 뽑기라 common이라도 귀한 것이 될 수 있습니다.
`pets den`으로 수집 현황을 봅니다.

생물은 에이전트의 것이라 그 세션이 살아 있는 동안만 존재합니다. 즉 세션을 시작할 때마다
다시 뽑는 셈입니다. 둥지는 안정적인 쪽으로, 같은 저장소와 worktree는 어떤 머신에서도
같은 도시가 됩니다. 아무것도 저장하지 않고서요. 그리고 둥지는 그곳에서 일했던
에이전트를 떠난 뒤에도 기억합니다.

## 명령어

| | |
|---|---|
| `pets party [--all]` | 살아 있는 worktree 전부, 나쁜 순으로 |
| `pets den` | 수집 현황 |
| `pets card [path]` | worktree 하나의 상세 |
| `pets render [--format=…]` | `statusline` / `tmux` / `title` / `json` |
| `pets install` / `pets uninstall` | 에이전트에 연결 / 해제 |
| `pets version` | |

## 지원 에이전트

| 에이전트 | 지원 범위 |
|---|---|
| Claude Code | 전체 지원. 상태줄, 부화, 한마디 |
| Codex CLI | 훅은 작성했으나 실제로 검증하지 못함. 펫 표시는 셸 스니펫으로 |
| tmux / 셸 프롬프트 | 전체 지원. **모든** 에이전트에서 동작 |
| 에디터 | `pets render --format=json`으로 직접 구현 |

tmux와 셸 방식은 에이전트에 아무것도 요구하지 않으므로 Aider, Amp, Gemini CLI 등에서도
동작합니다. 엔드투엔드로 검증된 것은 Claude Code뿐입니다. 다른 것에서 되거나 안 되면
issue를 남겨주시면 좋겠습니다.

## 라이선스

MIT. [LICENSE](LICENSE)를 참고하세요.

---

> 이 번역은 영어판을 바탕으로 한 기계 번역입니다. 어색한 표현이 있다면
> [issue](https://github.com/TevvvB/termagitchi/issues)나 PR로 고쳐주시면 감사하겠습니다.
