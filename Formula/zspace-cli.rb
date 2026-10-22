# 开发期配方。发布后由 goreleaser 写入 vangie/homebrew-tap，
# 安装命令变为 brew install vangie/tap/zspace-cli。
class ZspaceCli < Formula
  desc "极空间 NAS 的命令行与 MCP 服务"
  homepage "https://github.com/vangie/zspace-cli"
  url "https://github.com/vangie/zspace-cli/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000"
  license "MIT"

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X github.com/vangie/zspace-cli/internal/cli.version=#{version} -X github.com/vangie/zspace-cli/internal/mcp.version=#{version}"
    system "go", "build", *std_go_args(ldflags:, output: bin/"zspace"), "./cmd/zspace"
    system "go", "build", *std_go_args(ldflags:, output: bin/"zspace-mcp"), "./cmd/zspace-mcp"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/zspace version")
  end
end
