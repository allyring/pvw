package main

import (
	"fmt"
	"github.com/bastjan/netstat"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"os"

	"os/exec"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------------------------------------------------

// All the stuff relating to getting the processes with ports open
// Note that it's a bit buggy atm, a lot of 0s in fields where stuff can't be found.


func checkProcesses() tea.Msg {
	// Use the netstat package to get all the processes listening on ports
	conn, connErr := netstat.TCP.Connections()
	if (connErr != nil) {
		// Error if we fail, rather than running extra code
		return errMsg{connErr}
	}

	// Create a temporary map with process IDs mapped to a Process struct (used to merge processes with multiple ports
	// open).
	Pids := make(map[int]process)

	// We know we didn't fail, so we got ports successfully! Time to loop through them and add them to a Process slice
	for _, processWithPort := range conn {
		// Foreach loop that saves the element to the variable port and ignores the index (why don't more languages let
		// you save the index as well? That's so damn helpful!)

		// Take the process path and just get the binary's name by splitting at / and selecting the last item in the
		// array


		// Check if PID is not 0, if so, then ignore (in 'swap process'? idk what this it tbh, but google said so)
		if processWithPort.Pid != 0 {

			// If the process ID already has a port open (if processId in keys of PIDs)
			if _, ok := Pids[processWithPort.Pid]; ok {

				// Then add just the port from netstat to the Process.Ports slice
				tmpAppend := Pids[processWithPort.Pid]
				tmpAppend.ports = append(tmpAppend.ports, processWithPort.RemotePort)


				Pids[processWithPort.Pid] = tmpAppend

			} else {
				// else create a new Process struct with the info from netstat

				processPath := strings.Split(processWithPort.Exe, "/")
				processName := processPath[len(processPath) - 1]

				Pids[processWithPort.Pid] = process{id: processWithPort.Pid, name: processName, ports: []int{processWithPort.RemotePort}, userId: processWithPort.UserID}

			}
		}

	}

	allProcesses := make([]process,0)

	// We now have a map of all processes with ports open, so loop through all of them and add their value to the model
	for _, processWithPort := range Pids {
		// Add value to allProcesses
		allProcesses = append(allProcesses, processWithPort)
	}

	// return final array of processes as a processesMsg
	return processesMsg(allProcesses)


}

// Function to convert process struct to an array of arrays of strings (for use in rendering table)
func convertToRows(processes []process) []table.Row {
	final := make([]table.Row,0)

	for _, proc := range processes {
		individualRow := make(table.Row, 4)
		individualRow[0] = strconv.Itoa(proc.id)

		individualRow[1] = proc.name

		tmpPortStrings := make([]string,0)
		for _, port := range proc.ports {
			tmpPortStrings = append(tmpPortStrings, strconv.Itoa(port))
		}

		individualRow[2] = strings.Join(tmpPortStrings, ", ")

		individualRow[3] = proc.userId

		final = append(final, individualRow)

	}

	return final
}



// Func to create a command that will kill a given process ID
func terminateProcess(id int) tea.Cmd {
	return func() tea.Msg {
		pid := id

		cmd := exec.Command("kill", "-15", strconv.Itoa(pid))

		// Kill the process with that ID
		err := cmd.Run()
		if err != nil {
			return errMsg{err}
		}
		return killMsg{""}
	}
}

// The data returned
type process struct {
	id int
	name string
	ports []int
	userId string
}

// The types for messages returned for use with bubbletea's TUI
type processesMsg []process
type errMsg struct{ err error }
type killMsg struct{output string}

// Util function to get the error text from an errMsg element
func (e errMsg) Error() string { return e.err.Error() }


// ---------------------------------------------------------------------------------------------------------------------
// All the stuff relating to the bubbletea TUI

// All the keybinding // keymap stuff

// keyMap defines a set of keybindings. To work for help it must satisfy
// key.Map. It could also very easily be a map[string]key.Binding.
type keyMap struct {
	Up    key.Binding
	Down  key.Binding

	Refresh key.Binding
	Terminate key.Binding

	Help  key.Binding
	Quit  key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Refresh, k.Terminate, k.Help, k.Quit}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down,}, // first column
		{ k.Terminate, k.Refresh,},
		{ k.Help, k.Quit,}, // second column
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r", "u"),
		key.WithHelp("r/u", "refresh"),
	),
	Terminate: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "terminate"),
	),
}



// Start by creating a theme to use in rendering (edit later)
var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))


type model struct {
	// We're only storing info in the table, so just use that
	table table.Model
	procs []process
	err error

	// Help info
	keys       keyMap
	help       help.Model
	inputStyle lipgloss.Style

}

// We want to get the processes on first run, so send that on init

func (m model) Init() tea.Cmd {
	return checkProcesses
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// If we set a width on the help menu it can gracefully truncate
		// its view as needed.
		m.help.Width = msg.Width


	case tea.KeyMsg:
		switch msg.String() {


		case "/":
			m.help.ShowAll = !m.help.ShowAll
		case "q", "ctrl+c":
			return m, tea.Quit
		case "u", "r":
			return m, checkProcesses
		case "t":
			// get current process at the current cursor index
			if len(m.procs) > 0 {
				cursor := m.table.Cursor()
				return m, terminateProcess(m.procs[cursor].id)
			}
			return m, nil

		}

	case errMsg:
		m.err = msg
		return m, nil // Just ignore errors for now

	case processesMsg:
		// We have processes, lets update the model to use the new processes

		m.table.SetRows(convertToRows(msg)) // Convert the array of process structs to text for use in rendering

		m.procs = msg // Table doesn't allow you to get rows >:( pls merge that pull request charm, ty :)
		return m, nil

	case killMsg:
		// kill process worked? Rerender processes
		return m, checkProcesses
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	helpView := m.help.View(m.keys)
	tableString := baseStyle.Render(m.table.View())
	height := 2 - strings.Count(helpView, "\n")

	final := tableString + strings.Repeat("\n", height) + helpView
	if m.err != nil {
		final += "Error: " + m.err.Error()
	}
	return final
}

func main() {
	columns := []table.Column{
		{Title: "PID", Width: 5},
		{Title: "Name", Width: 24},
		{Title: "Ports", Width: 16},
		{Title: "User ID", Width: 6},
	}

	rows := []table.Row{}
	// Set to empty, then let render fill them out

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(7),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	m := model{t, []process{}, nil, keys, help.New(), baseStyle}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

}


// PUT THIS IN MAIN
//if runtime.GOOS == "windows" {
//fmt.Println("Can't Execute this on a windows machine")
//} else {
//execute()
//}
