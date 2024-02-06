package proxycore

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/twmb/murmur3"
)

const NetworkTopologyStrategy = "NetworkTopologyStrategy"
const SimpleStrategy = "SimpleStrategy"

type Token interface {
	fmt.Stringer
	LessThan(Token) bool
}

type Partitioner interface {
	fmt.Stringer
	Hash(partitionKey []byte) Token
	FromString(token string) Token
}

type TokenHost struct {
	Token
	*Host
}

type TokenReplicas struct {
	Token
	Replicas []*Host
}

type Datacenter struct {
	numNodes int
	racks    map[string]struct{}
}

type ReplicationStrategy interface {
	BuildTokenMap(tokens []TokenHost, dcs map[string]*Datacenter) []TokenReplicas
	Key() string
}

type TokenMap struct {
	hosts         map[string]*Host
	dcs           map[string]*Datacenter
	tokens        []TokenHost
	keyspaces     map[string]ReplicationStrategy
	tokenReplicas map[string][]TokenReplicas // Uses replication strategy Key()
	rwMutex       sync.RWMutex
	updateMu      sync.Mutex // Single updater
}

func NewTokenMap(hosts []*Host, keyspaces map[string]ReplicationStrategy) *TokenMap {
	tokens := make([]TokenHost, 0)
	hostsMap := make(map[string]*Host)

	for _, host := range hosts {
		hostsMap[host.Key()] = host
		for _, token := range host.Tokens {
			tokens = append(tokens, TokenHost{
				Token: token,
				Host:  host,
			})
		}
	}

	sortTokens(tokens)

	dcs := buildDcs(hostsMap)

	return &TokenMap{
		hosts:         hostsMap,
		dcs:           dcs,
		tokens:        tokens,
		keyspaces:     keyspaces,
		tokenReplicas: buildTokenReplicas(tokens, dcs, keyspaces),
	}
}

func (t *TokenMap) GetReplicas(keyspace string, token Token) (replicas []*Host, err error) {
	t.rwMutex.RLock()
	defer t.rwMutex.RUnlock()
	if rs, ok := t.keyspaces[keyspace]; ok {
		tokenReplicas := t.tokenReplicas[rs.Key()]
		index := sort.Search(len(tokenReplicas), func(i int) bool { return token.LessThan(tokenReplicas[i]) })
		if index < 0 {
			return tokenReplicas[0].Replicas, nil
		} else {
			return tokenReplicas[index].Replicas, nil
		}
	} else {
		return nil, fmt.Errorf("'%s' keyspace does not exist in token map", keyspace)
	}
}

func (t *TokenMap) AddHost(host *Host) {
	t.updateMu.Lock()
	defer t.updateMu.Unlock()

	tokensCopy := make([]TokenHost, len(t.tokens))

	for _, token := range host.Tokens {
		tokensCopy = append(tokensCopy, TokenHost{
			Token: token,
			Host:  host,
		})
	}

	t.hosts[host.Key()] = host
	t.dcs = buildDcs(t.hosts)

	sortTokens(tokensCopy)

	tokenReplicasCopy := buildTokenReplicas(t.tokens, t.dcs, t.keyspaces)

	t.rwMutex.Lock()
	t.tokens = tokensCopy
	t.tokenReplicas = tokenReplicasCopy
	t.rwMutex.Unlock()
}

func (t *TokenMap) RemoveHost(host *Host) {
	t.updateMu.Lock()
	defer t.updateMu.Unlock()

	tokensCopy := make([]TokenHost, 0)

	for _, tokenHost := range t.tokens {
		if tokenHost.Host != host && tokenHost.Host.Key() != host.Key() {
			tokensCopy = append(tokensCopy, tokenHost)
		}
	}

	delete(t.hosts, host.Key())
	t.dcs = buildDcs(t.hosts)

	sortTokens(tokensCopy)

	tokenReplicasCopy := buildTokenReplicas(t.tokens, t.dcs, t.keyspaces)

	t.rwMutex.Lock()
	t.tokens = tokensCopy
	t.tokenReplicas = tokenReplicasCopy
	t.rwMutex.Unlock()
}

func (t *TokenMap) AddKeyspace(keyspace string, rs ReplicationStrategy) {
	t.updateMu.Lock()
	defer t.updateMu.Unlock()

	if _, ok := t.tokenReplicas[rs.Key()]; !ok {
		tokenMap := rs.BuildTokenMap(t.tokens, t.dcs)

		t.rwMutex.Lock()
		t.keyspaces[keyspace] = rs
		t.tokenReplicas[rs.Key()] = tokenMap
		t.rwMutex.Unlock()
	} else {
		t.rwMutex.Lock()
		t.keyspaces[keyspace] = rs
		t.rwMutex.Unlock()
	}
}

func buildTokenReplicas(tokens []TokenHost, dcs map[string]*Datacenter, keyspaces map[string]ReplicationStrategy) map[string][]TokenReplicas {
	tokenReplicas := make(map[string][]TokenReplicas)
	for _, rs := range keyspaces {
		if _, ok := tokenReplicas[rs.Key()]; !ok {
			tokenReplicas[rs.Key()] = rs.BuildTokenMap(tokens, dcs)
		}
	}
	return tokenReplicas
}

func buildDcs(hosts map[string]*Host) map[string]*Datacenter {
	dcs := make(map[string]*Datacenter)
	for _, host := range hosts {
		if dc, ok := dcs[host.DC]; ok {
			dc.racks[host.Rack] = struct{}{}
			dc.numNodes++
		} else {
			dcs[host.DC] = &Datacenter{
				numNodes: 1,
				racks:    make(map[string]struct{}),
			}
		}
	}
	return dcs
}

func sortTokens(tokens []TokenHost) {
	sort.SliceStable(tokens, func(i, j int) bool {
		return tokens[i].LessThan(tokens[j])
	})
}

type murmur3Token struct {
	hash int64
}

func (m murmur3Token) String() string {
	return strconv.FormatInt(m.hash, 10)
}

func (m murmur3Token) LessThan(token Token) bool {
	if t, ok := token.(*murmur3Token); ok {
		return m.hash < t.hash
	} else {
		panic("tried comparing incompatible token types")
	}
}

func NewPartitionerFromName(name string) (Partitioner, error) {
	if strings.EqualFold(name, "Murmur3Partitioner") {
		return NewMurmur3Partitioner(), nil
	} else {
		return nil, fmt.Errorf("'%s' is an unsupported paritioner", name)
	}
}

type murmur3Partitioner struct {
}

func NewMurmur3Partitioner() Partitioner {
	return &murmur3Partitioner{}
}

func (m murmur3Partitioner) String() string {
	return "Murmur3Partitioner"
}

func (m murmur3Partitioner) Hash(partitionKey []byte) Token {
	return &murmur3Token{int64(murmur3.Sum64(partitionKey))}
}

func (m murmur3Partitioner) FromString(token string) Token {
	hash, _ := strconv.ParseInt(token, 10, 64) // TODO: Don't ignore error
	return &murmur3Token{hash}
}

type simpleReplicationStrategy struct {
	replicationFactor int
	key               string
}

func (s simpleReplicationStrategy) BuildTokenMap(tokens []TokenHost, _ map[string]*Datacenter) []TokenReplicas {
	numReplicas := s.replicationFactor
	numTokens := len(tokens)
	result := make([]TokenReplicas, 0, numTokens)

	if numTokens < numReplicas {
		numReplicas = numTokens
	}

	for i, token := range tokens {
		replicas := make([]*Host, 0, numReplicas)
		for j := 0; j < numTokens && len(replicas) < numReplicas; j++ {
			replicas = append(replicas, tokens[i].Host)
			i++
			if i >= numTokens {
				i = 0
			}
		}
		result = append(result, TokenReplicas{token, replicas})
	}

	return result
}

func (s simpleReplicationStrategy) Key() string {
	return s.key
}

type networkTopologyReplicationStrategy struct {
	dcReplicationFactors map[string]int
	key                  string
}

type dcState struct {
	skippedEndpoints []*Host
	racksObserved    map[string]struct{}
	replicaCount     int
}

type dcInfo struct {
	replicationFactor int
	numRacks          int
}

func appendReplica(replicas []*Host, replicaCountThisDc int, replicaToAdd *Host) ([]*Host, int) {
	for _, replica := range replicas {
		if replica == replicaToAdd || replica.Key() == replicaToAdd.Key() {
			return replicas, replicaCountThisDc
		}
	}
	replicaCountThisDc++
	return append(replicas, replicaToAdd), replicaCountThisDc
}

func (n networkTopologyReplicationStrategy) BuildTokenMap(tokens []TokenHost, dcs map[string]*Datacenter) []TokenReplicas {
	infos := make(map[string]dcInfo)

	numTokens := len(tokens)
	result := make([]TokenReplicas, 0, numTokens)

	numReplicas := 0

	for dcName, rf := range n.dcReplicationFactors {
		if dc, ok := dcs[dcName]; ok {
			numReplicas += rf
			infos[dcName] = dcInfo{
				replicationFactor: rf,
				numRacks:          len(dc.racks),
			}
		}
	}

	if numReplicas == 0 {
		return result
	}

	for i, token := range tokens {
		replicas := make([]*Host, 0, numReplicas)
		states := make(map[string]*dcState)

		for j := 0; j < numTokens && len(replicas) < numReplicas; j++ {
			host := tokens[i].Host

			// Move to the next token, we got the host for the current token in the previous step
			i++
			if i >= numTokens { // Wrap to the first token
				i = 0
			}

			if info, ok := infos[host.DC]; !ok {
				continue // Not a valid datacenter, go to the next token
			} else {
				var state *dcState
				if state, ok = states[host.DC]; !ok {
					state = &dcState{
						skippedEndpoints: nil,
						racksObserved:    make(map[string]struct{}),
						replicaCount:     0,
					}
					states[host.DC] = state
				}

				if state.replicaCount >= info.replicationFactor {
					continue
				}

				if len(host.Rack) == 0 || len(state.racksObserved) == info.numRacks {
					replicas, state.replicaCount = appendReplica(replicas, state.replicaCount, host)
				} else {
					if _, ok = state.racksObserved[host.Rack]; ok {
						state.skippedEndpoints = append(state.skippedEndpoints, host)
					} else {
						replicas, state.replicaCount = appendReplica(replicas, state.replicaCount, host)
						state.racksObserved[host.Rack] = struct{}{} // Observe the rack

						if len(state.racksObserved) == info.numRacks {
							for len(state.skippedEndpoints) > 0 && state.replicaCount < info.replicationFactor {
								replicas, state.replicaCount = appendReplica(replicas, state.replicaCount, host)
								state.skippedEndpoints = state.skippedEndpoints[1:]
							}
						}
					}
				}
			}
		}
		result = append(result, TokenReplicas{token, replicas})
	}

	return result
}

func (n networkTopologyReplicationStrategy) Key() string {
	return n.key
}

func NewReplicationFactor(row Row) (ReplicationStrategy, error) {
	replicationFactors := make(map[string]int)
	replicationColumn, err := row.ByName("replication")
	var class string
	if err == ColumnNameNotFound {
		strategyClass, err := row.ByName("strategy_class")
		if err != nil {
			return nil, errors.New("couldn't find 'strategy_class' column in keyspace metadata")
		}
		class = strategyClass.(string)
		strategyOptions, err := row.ByName("strategy_options")
		if err != nil {
			return nil, errors.New("couldn't find 'strategy_options' column in keyspace metadata")
		}
		options := make(map[string]string)
		err = json.Unmarshal([]byte(strategyOptions.(string)), &options)
		if err != nil {
			return nil, fmt.Errorf("'strategy_options' column is invalid: %v", err)
		}
		for k, v := range options {
			switch k {
			case "replication_factor":
				rf, err := strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Errorf("invalid replication factor: %s. Expected an integer value", v)
				}
				replicationFactors["rf"] = rf
			default:
				rf, err := strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Errorf("invalid replication factor: %s. Expected an integer value", v)
				}
				replicationFactors[k] = rf // Key should be a data center
			}
		}
	} else {
		replication := replicationColumn.(map[string]string)
		for k, v := range replication {
			switch k {
			case "class":
				class = v
			case "replication_factor":
				rf, err := strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Errorf("invalid replication factor: %s. Expected an integer value", v)
				}
				replicationFactors["rf"] = rf
			default:
				rf, err := strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Errorf("invalid replication factor: %s. Expected an integer value", v)
				}
				replicationFactors[k] = rf // Key should be a data center
			}
		}
	}

	if strings.EqualFold(class, NetworkTopologyStrategy) {
		return &networkTopologyReplicationStrategy{
			dcReplicationFactors: replicationFactors,
			key:                  fmt.Sprintf("%v", replicationFactors),
		}, nil
	} else if strings.EqualFold(class, SimpleStrategy) {
		return &simpleReplicationStrategy{
			replicationFactor: replicationFactors["rf"],
			key:               fmt.Sprintf("%d", replicationFactors["rf"]),
		}, nil
	} else {
		return nil, fmt.Errorf("invalid replication strategy: '%s'", class)
	}
}
