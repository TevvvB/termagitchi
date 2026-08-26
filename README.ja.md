<p align="center">
  <img src="docs/header.png" alt="termagitchi" width="100%" />
</p>

<p align="center">
  <strong>エージェントごとに一匹のいきもの、git worktree ごとに一つの巣（den）。</strong>
</p>

<p align="center">
  <a href="README.md">English</a> ·
  <b>日本語</b> ·
  <a href="README.ko.md">한국어</a> ·
  <a href="README.zh-Hans.md">简体中文</a>
</p>

<p align="center">
  よくあるターミナルペットは <i>あなた</i> のものです。一匹を長く育てる。これは違います。
  ここでのペットは <b>エージェント</b> のものです。worktree はそれぞれ名前を持つ「巣」になり、
  そこで動いているエージェントは一匹ずつ自分のいきものを持ちます。機嫌はその worktree の
  きれいさを映します。エージェントが六つ動いていれば、いきものも六匹同時に生きている。
  一目で見分けがつくので、どのセッションを見ているのか、どれが困っているのかがすぐ分かります。
</p>

<p align="center">
  <img src="docs/demo.gif" alt="六つの worktree、六匹のいきもの" width="100%" />
</p>

## インストール

**Linux と macOS**

```sh
curl -fsSL https://raw.githubusercontent.com/TevvvB/termagitchi/main/install.sh | sh
pets install
```

**macOS（Homebrew）**

```sh
brew install TevvvB/tap/termagitchi
pets install
```

**Windows** — [Releases](https://github.com/TevvvB/termagitchi/releases)
からアーカイブを取得し、`pets` を `PATH` に置いてから `pets install` を実行してください。

**Go を使う場合** — `go install github.com/TevvvB/termagitchi/cmd/pets@latest`

`pets install` はインストール済みのエージェントを見つけて自分を組み込みます。触れた設定は
バックアップし、それ以外の設定はそのまま残します。実行後は新しいセッションを開いてください。
`pets uninstall` で元に戻せます。

エージェントを入れただけで一度も起動していない場合は設定ディレクトリがまだ無いので、
`pets install --harness=claude` のように直接指定してください。

## 読み方

| 記号 | 意味 |
|---|---|
| `@ DXB` | 巣（den）。どの worktree かを空港コードで表す |
| `•ᴗ•` `•_•` `¬_¬` `>_<` `@_@` `x_x` | 機嫌。ハート5から0まで |
| `7△` | 未コミット 7 ファイル |
| `3↑` | 未プッシュ 3 コミット |
| `105↓` | trunk からの遅れ。40 を超えたときだけ表示 |
| `⑂2` | マイグレーションの head が 2 つ |
| `✗` | 直近のテストまたは lint が失敗 |
| `-.-` と `·····` | 起動直後、まだ目覚めていない状態 |

ハートは 5 から始まり、未コミットのファイルがあれば 1 減り、15 を超えるとさらに 1、
未プッシュのコミットがあれば 1、5 を超えるとさらに 1、テストが落ちていれば 2 減ります。
すべて設定で変えられます。

テスト結果はランナーが明示的に結果を告げたときだけ記録され、2 時間を過ぎた結果は忘れられます。
すでに直したブランチに赤い印が残り続けることはありません。

## すべての worktree を一度に見る

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

状態の悪いものが上に来るので、手当てが必要なものがいちばん目立ちます。エージェントがいる巣には
その顔ぶれがセッション名つきで並ぶので、同じ worktree を共有する二つのエージェントは
一匹ではなく二匹として表示されます。誰もいない巣にもいきものはいます。
一覧が長いときは省略され、`pets party --all` ですべて表示できます。

## 集める

これはガチャです。いきものはセッションごとに孵るので、worktree を新しく開くことがそのまま
一回分の抽選になります。レアリティは五段階です。

| 段階 | 確率 | |
|---|---|---|
| common | 60% | `✦` |
| uncommon | 25% | `✦✦` |
| rare | 11% | `✦✦✦` |
| legendary | 3.5% | `✦✦✦✦` |
| mythic | 0.5% | `✦✦✦✦✦` |

rare 以上はその段階の色をまとうので、星を数えなくても何を引いたか分かります。
common と uncommon には独自の色相が割り当てられ、同じ種類でも見分けられます。

色違い（shiny）は 1/128 の独立した抽選なので、common でも「当たり」になり得ます。
`pets den` でコレクションを確認できます。

いきものはエージェントのものなので、そのセッションが続くあいだだけ生きています。
つまりセッションを始めるたびに引き直しです。巣のほうは安定していて、同じリポジトリと
worktree はどのマシンでも同じ都市になります。何も保存せずにです。そして巣は、
そこで働いたエージェントを去ったあとも覚えています。

## コマンド

| | |
|---|---|
| `pets party [--all]` | 生きている worktree すべてを、悪い順に |
| `pets den` | コレクション |
| `pets card [path]` | worktree 一つの詳細 |
| `pets render [--format=…]` | `statusline` / `tmux` / `title` / `json` |
| `pets install` / `pets uninstall` | エージェントへの組み込みと解除 |
| `pets version` | |

## 対応エージェント

| エージェント | できること |
|---|---|
| Claude Code | フル対応。ステータスライン、孵化、ひとこと |
| Codex CLI | フックは書いてあるが実機未検証。ペット表示は下のシェル用スニペットで |
| tmux / シェルプロンプト | フル対応。**どのエージェントでも**動く |
| エディタ | `pets render --format=json` で自作 |

tmux とシェルの方法はエージェント側に何も要求しないので、Aider でも Amp でも Gemini CLI でも
動きます。エンドツーエンドで検証できているのは Claude Code だけです。ほかのもので動いた、
あるいは動かなかった場合は issue を立ててもらえると助かります。

## ライセンス

MIT。[LICENSE](LICENSE) を参照してください。

---

> この翻訳は英語版からの機械翻訳をもとにしています。不自然な表現があれば
> [issue](https://github.com/TevvvB/termagitchi/issues) や PR で直してもらえると助かります。
