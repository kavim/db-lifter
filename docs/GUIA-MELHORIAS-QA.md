# Guia de melhorias (visão de QA) — db-lift

Este texto é para **quem quer entender o projeto e o que o tornaria mais profissional**, mesmo com pouca experiência em Go. Onde fizer sentido, explicamos **o que é** cada peça em linguagem simples.

---

## O que é este projeto (em uma frase)

O **db-lift** é um programa de linha de comando (CLI) escrito em **Go**: lê um ficheiro `.sql` grande e envia esse conteúdo para o MySQL que corre **dentro de um contentor Docker**, mostrando uma interface “bonita” no terminal (barra de progresso, spinners).

Em Go, a estrutura típica é:

- **`cmd/…`** — ponto de entrada (o `main` que corre quando executas o binário).
- **`internal/…`** — código interno da aplicação (não é feito para ser importado por outros projetos como biblioteca).

Este projeto segue esse padrão.

---

## O que já está bem feito

| Aspeto | Porquê importa |
|--------|----------------|
| **Pastas `cmd` e `internal`** | Organização padrão em Go; facilita ler e manter o código. |
| **Mensagens de erro com `%w`** | Em Go, `fmt.Errorf("…: %w", err)` “embrulha” o erro original para ferramentas e `errors.Is` / `errors.As` funcionarem — é boa prática. |
| **Contador de bytes com `atomic`** | Várias partes do programa podem ler o progresso ao mesmo tempo; `atomic` evita condições de corrida sem um `mutex` mais pesado. |
| **Validação de flags obrigatórias** | O utilizador não chega ao meio do restore sem ficheiro, contentor ou base de dados. |
| **Build estático no Makefile** | `CGO_ENABLED=0` gera um binário que costuma ser mais fácil de copiar para outras máquinas (menos dependências do sistema). |

Nada disto “precisa de melhoria” por si só; são bases sólidas.

---

## Melhorias sugeridas (explicadas para quem está a começar em Go)

### 1. Testes automatizados (`*_test.go`)

**O que são:** Em Go, ficheiros que terminam em `_test.go` contêm funções `TestXxx(t *testing.T)`. Corres `go test ./...` e o Go executa esses testes.

**Porquê falta hoje:** O projeto não tem testes. Para uma ferramenta que **apaga e recria bases de dados**, testes dão confiangça de que mudanças no código não partem comportamentos críticos.

**O que adicionar primeiro (pedagogicamente fácil):**

- Testes **unitários** para pacotes que não precisam de Docker — por exemplo o pacote `progress` (ler dados de um `strings.Reader` e verificar se os bytes contados batem certo).
- Mais tarde: integração com Docker só em CI ou ambiente controlado.

**Conceito Go:** `testing.T`, `t.Errorf`, table-driven tests (uma tabela de casos num slice e um `for`).

---

### 2. Integração contínua (CI)

**O que é:** Um serviço (por exemplo GitHub Actions) que, em cada `push` ou `pull request`, corre automaticamente comandos como `go test`, `go vet`, `golangci-lint`, `go build`.

**Porquê importa:** Sem CI, ninguém garante que o repositório “sempre compila e passa testes” após uma alteração.

**Para iniciantes em Go:** Não é sintaxe de Go — é “automação à volta do Go”. O Makefile já tem `make lint`; o CI só chama esses alvos de forma repetível.

---

### 3. Versão da aplicação (`--version`) e `ldflags`

**O problema:** O `Makefile` tenta injetar uma variável com `-X main.version=…`, mas em Go isso **só funciona se existir** no pacote `main` algo como:

```go
var version = "dev"
```

Sem essa variável, o link não associa o valor esperado (conforme a versão do Go, pode nem fazer o que pretendemos).

**O que esperamos num CLI profissional:** O utilizador corre `db-lift --version` e vê a versão (e idealmente o commit Git).

**Conceito Go:** `spf13/cobra` suporta `Version` no comando raiz; `ldflags` sobrescreve `main.version` no momento da compilação.

---

### 4. Cancelar o restore ao sair da interface (tecla `q`)

**Contexto:** O programa usa `context.Context` — em Go, isso é a forma idiomática de dizer “esta operação deve parar” (por timeout, sinal, ou decisão do utilizador).

**Hoje:** O cancelamento está ligado a sinais do sistema (`SIGINT`, `SIGTERM`). Se o utilizador **só** fechar a TUI com `q`, o `context` pode **não** ser cancelado e o restore continua em segundo plano.

**Melhoria:** Quando a TUI decidir terminar, deve disparar o **mesmo** `cancel()` que os sinais usam.

**Conceito Go:** `context.WithCancel`; guardar `cancel` e chamá-lo de um sítio único quando qualquer “sair” acontecer.

---

### 5. Timeout global (`--timeout`)

**O que é:** `context.WithTimeout(ctx, d)` cria um contexto que **cancela automaticamente** após um tempo máximo.

**Porquê:** Restores gigantes ou rede presa podem parecer “travados”. Um timeout dá fim controlado e mensagem clara.

**Para iniciantes:** Timeout não “mata o CPU” magicamente — ao cancelar o `context`, o código que respeita `ctx` deve deixar de esperar e fechar processos filhos (por exemplo o `docker exec`). Verificar se ao cancelar o `exec.CommandContext` o processo externo é realmente terminado pode exigir testes manuais ou cuidado extra.

---

### 6. Palavras-passe e linha de comando (segurança)

**O problema:** Passar a password como `-palgo` na string do `mysql` pode fazer com que o valor apareça em listagens de processos (`ps`) no sistema anfitrião ou no contentor.

**Melhoria (conceitual):** Métodos que reduzem exposição — variáveis de ambiente **só dentro** do exec, ficheiros de credenciais temporários com permissões restritas, ou documentar claramente o risco e boas práticas.

Isto **não é específico de Go**; é prática de operações e segurança.

---

### 7. Barra de progresso quando o ficheiro “não tem tamanho fiável”

**O que acontece:** O código usa o tamanho do ficheiro (`Stat().Size()`) para a percentagem. Em alguns casos (pipes, certos dispositivos), o tamanho pode ser **0** ou não corresponder ao que vais ler.

**Melhoria:** Se o ficheiro não for um ficheiro normal (`Mode().IsRegular()`), mostrar apenas bytes transferidos ou uma barra “indeterminada”, em vez de percentagem enganosa.

**Conceito Go:** `os.FileInfo` e `fs.FileMode` vêm do pacote `os` / `io/fs`.

---

### 8. Modo sem TUI (`--no-tui` ou deteção de CI)

**Porquê:** Em scripts, pipes ou ambientes sem terminal “real”, uma TUI pode atrapalhar ou não funcionar bem.

**Melhoria:** Caminho de execução que só imprime texto simples (e talvez nível de log), ativado por flag ou quando `stdout` não é um TTY (`isatty`).

**Conceito Go:** `os.Stdout` pode ser inspecionado com pacotes como `github.com/mattn/go-isatty` (o projeto já usa `mattn/go-isatty` indirectamente via Charm).

---

### 9. Ficheiro de configuração do `golangci-lint`

**O que é:** `.golangci.yml` na raiz define **regras de lint** (estilo, erros comuns, imports).

**Porquê:** `make lint` passa a ser **reprodutível** entre máquinas: todos correm as mesmas regras.

---

### 10. `.env.example` em vez de commitar segredos

O `.gitignore` já ignora `.env` — correto.

**Boa prática:** Ter um `.env.example` com chaves **sem valores secretos**, para novos contribuidores copiarem para `.env`.

---

## Glossário rápido (Go e termos do projeto)

| Termo | Significado breve |
|------|-------------------|
| **Pacote (`package`)** | Unidade de organização; nome da pasta costuma ser o nome do pacote (`restore`, `docker`, …). |
| **`go.mod` / `go.sum`** | Declaram o módulo e as dependências com versões fixas; `go.sum` garante integridade dos downloads. |
| **`internal/`** | Convenção Go: só código **deste** módulo pode importar `…/internal/…` de forma estável. |
| **Context** | Propaga cancelamento e deadlines para operações longas. |
| **`defer`** | “Corre isto quando a função acabar” (útil para `f.Close()`). |
| **Goroutine** | Função que corre em paralelo leve (`go func() { … }()`). |
| **`chan` (canal)** | Fila segura para comunicar entre goroutines; aqui transporta `Status` da restore para a TUI. |
| **Cobra** | Biblioteca popular para subcomandos, flags e help em CLI Go. |
| **Bubble Tea** | Framework de TUI; “Model–Update–View”, parecido com ideias de Elm. |

---

## Resumo em tabela

| Área | Estado sugerido na análise | Ideia principal |
|------|---------------------------|-----------------|
| Testes | Ausentes | Começar por `progress` e funções puras |
| CI | Ausente | Automatizar `test`, `vet`, `lint`, `build` |
| Versão | Incompleta | `var version` + `--version` + alinhar `Makefile` |
| Cancelamento | Parcial | Cancelar `context` também ao sair pela TUI |
| Timeout | Opcional na spec | `context.WithTimeout` + mensagem clara |
| Segurança | Atenção à password | Menos exposição em `argv` / documentar |
| Progresso | Edge cases | Ficheiros não regulares / tamanho 0 |
| Automação | TUI só | Modo `--no-tui` ou deteção de TTY |
| Lint | Sem config versionada | `.golangci.yml` |
| Repositório | Boas práticas | `.env.example`, sem segredos |

---

## Leitura oficial útil (inglês)

- [Go tutorial](https://go.dev/doc/tutorial/getting-started) — primeiro programa e módulos.
- [How to write Go code](https://go.dev/doc/code) — layout de projeto.
- [Testing](https://go.dev/doc/tutorial/add-a-test) — testes básicos.

Este ficheiro **não substitui** o README de utilizador; complementa-o com uma visão de **qualidade, riscos e aprendizagem** para quem mantém ou evolui o código.
