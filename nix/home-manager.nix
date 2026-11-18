self:
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.programs.jt;
  yaml = pkgs.formats.yaml { };

  # Drop unset values so jt falls back to its own defaults for them.
  settings = lib.filterAttrsRecursive (_: v: v != null) cfg.settings;
  settingsFile = yaml.generate "jt.yaml" settings;

  configDir =
    if config.xdg.enable then config.xdg.configHome else "${config.home.homeDirectory}/.config";
  configPath = "${configDir}/jt/jt.yaml";
in
{
  options.programs.jt = {
    enable = lib.mkEnableOption "jt, a terminal kanban board for Jira";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
      defaultText = lib.literalExpression "jt.packages.\${pkgs.stdenv.hostPlatform.system}.default";
      description = "The jt package to install.";
    };

    mutableSettings = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = ''
        Whether jt can write to its settings file. jt saves the theme, default
        project and board there, so a read-only file makes those changes fail.

        When true, each activation merges {option}`programs.jt.settings` into
        the existing file, which stays a regular, writable file. Declared
        values win, and keys you don't declare (such as a theme picked in jt)
        are kept.

        When false, the file is a read-only link into the Nix store and jt
        can't save anything.
      '';
    };

    settings = lib.mkOption {
      type = lib.types.submodule {
        freeformType = yaml.type;
        options = {
          theme = lib.mkOption {
            type = lib.types.nullOr lib.types.str;
            default = null;
            example = "dracula";
            description = ''
              ID of the bubbletint tint to draw with. Unset follows the
              terminal's theme.
            '';
          };

          icons = lib.mkOption {
            type = lib.types.nullOr lib.types.bool;
            default = null;
            description = "Whether to show Nerd Font icons. jt defaults to true.";
          };

          commands = lib.mkOption {
            type = lib.types.nullOr (
              lib.types.listOf (
                lib.types.submodule {
                  options = {
                    name = lib.mkOption {
                      type = lib.types.str;
                      default = "";
                      description = "Name shown in the command picker. Defaults to the command.";
                    };
                    command = lib.mkOption {
                      type = lib.types.str;
                      description = "The executable to run.";
                    };
                    args = lib.mkOption {
                      type = lib.types.listOf lib.types.str;
                      default = [ ];
                      description = "Arguments passed to the command.";
                    };
                  };
                }
              )
            );
            default = null;
            description = "User-defined commands available from the board.";
          };

          jira = {
            server = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              example = "https://example.atlassian.net";
              description = "Jira Cloud site URL.";
            };

            login = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              example = "you@example.com";
              description = ''
                Email address to sign in with. The API token isn't stored in
                the settings file. Run `jt init` to save it to the OS keyring.
              '';
            };

            project = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              example = "ABC";
              description = "Key of the project to open on start.";
            };

            board_id = lib.mkOption {
              type = lib.types.nullOr lib.types.int;
              default = null;
              example = 42;
              description = "Board to open for the project. Unset lets jt pick one.";
            };
          };
        };
      };
      default = { };
      example = lib.literalExpression ''
        {
          theme = "dracula";
          jira = {
            server = "https://example.atlassian.net";
            login = "you@example.com";
            project = "ABC";
          };
          commands = [
            { name = "View Issue in nvim"; command = "bash"; args = [ "-c", "echo '{{.Title}}\n{{.Description}}' > nvim" ]; }
          ];
        }
      '';
      description = ''
        Contents of jt's settings file, `jt/jt.yaml` in the XDG config
        directory. Any other key jt reads can be set too.
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];

    xdg.configFile."jt/jt.yaml" = lib.mkIf (!cfg.mutableSettings) {
      source = settingsFile;
    };

    home.activation.jtSettings = lib.mkIf cfg.mutableSettings (
      lib.hm.dag.entryAfter [ "writeBoundary" ] ''
        target=${lib.escapeShellArg configPath}
        run mkdir -p "$(dirname "$target")"

        # An earlier read-only setup leaves a store symlink behind.
        if [ -L "$target" ]; then
          run cp --remove-destination "$(readlink -f "$target")" "$target"
          run chmod 600 "$target"
        fi

        if [ -f "$target" ]; then
          tmp="$(mktemp)"
          ${lib.getExe pkgs.yq-go} eval-all \
            'select(fileIndex == 0) * select(fileIndex == 1)' \
            "$target" ${settingsFile} > "$tmp"
          run install -m 600 "$tmp" "$target"
          rm -f "$tmp"
        else
          run install -m 600 ${settingsFile} "$target"
        fi
      ''
    );
  };
}
