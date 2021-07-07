package manager

import (
	"encoding/json"
	"net/http"
)

const (
	LIST_PEER            = "list_peer"
	JOIN_SQUAD           = "join_squad"
	LIST_SQUADS          = "list_squads"
	SQUAD_ACCESS_DENIED  = "squad_access_denied"
	SQUAD_ACCESS_GRANTED = "squad_access_granted"
	LEAVE_SQUAD          = "leave_squad"
	SQUAD_AUTH           = "auth_squad"
	CREATE_SQUAD         = "create_squad"
	DELETE_SQUAD         = "delete_squad"
	MODIFY_SQUAD         = "modify_squad"
)

type SquadHTTPMiddleware struct{}

func (shm *SquadHTTPMiddleware) Process(r *ServRequest, req *http.Request, w http.ResponseWriter, m *Manager) (err error) {
	switch r.Type {
	case LIST_PEER:
		peers := []*Peer{}
		for id := range m.GRPCPeers {
			peers = append(peers, &Peer{
				Id:   id,
				Name: "unknow",
			})
		}
		for id := range m.WSPeers {
			peers = append(peers, &Peer{
				Id:   id,
				Name: "unknow",
			})
		}
		err = json.NewEncoder(w).Encode(peers)
		return
	case JOIN_SQUAD:
		if _, ok := r.Payload["squadId"]; !ok {
			http.Error(w, "no field squadId in payload", http.StatusBadRequest)
			return
		}
		if _, ok := r.Payload["password"]; !ok {
			http.Error(w, "no field password in payload", http.StatusBadRequest)
			return
		}
		if err = m.ConnectToSquad(r.Payload["squadId"], r.From, r.Payload["password"], false); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		err = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"squadId": r.Payload["squadId"],
		})
	case LIST_SQUADS:
		squads := []*Squad{}
		for _, squad := range m.Squads {
			squads = append(squads, squad)
		}
		err = json.NewEncoder(w).Encode(squads)
	case LEAVE_SQUAD:
		if _, ok := r.Payload["squadId"]; !ok {
			http.Error(w, "no field squadId in payload", http.StatusBadRequest)
			return
		}
		if err = m.LeaveSquad(r.Payload["squadId"], r.From, false); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"squadId": r.Payload["squadId"],
		})
	case SQUAD_AUTH:
	case CREATE_SQUAD:
		if _, ok := r.Payload["squadId"]; !ok {
			http.Error(w, "no field squadId in payload", http.StatusBadRequest)
			return
		}
		if _, ok := r.Payload["password"]; !ok {
			http.Error(w, "no field password in payload", http.StatusBadRequest)
			return
		}
		if _, ok := r.Payload["squadType"]; !ok {
			http.Error(w, "no field squadType in payload", http.StatusBadRequest)
			return
		}
		if _, ok := r.Payload["squadName"]; !ok {
			http.Error(w, "no field squadName in payload", http.StatusBadRequest)
			return
		}
		if _, ok := r.Payload["squadNetworkType"]; !ok {
			http.Error(w, "no field squadNetworkType in payload", http.StatusBadRequest)
			return
		}
		if r.Payload["squadNetworkType"] == HOSTED {
			if _, ok := r.Payload["squadHost"]; !ok {
				http.Error(w, "no field squadHost in payload", http.StatusBadRequest)
				return
			}
		}
		if err = m.CreateSquad(r.Payload["squadId"], r.From, r.Payload["squadName"], SquadType(r.Payload["squadType"]), r.Payload["password"], r.Payload["squadNetworkType"], r.Payload["squadHost"]); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})
	case DELETE_SQUAD:
		if _, ok := r.Payload["squadId"]; !ok {
			http.Error(w, "no field squadId in payload", http.StatusBadRequest)
			return
		}
		if err = m.DeleteSquad(r.Payload["squadId"], r.From); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})
	case MODIFY_SQUAD:
		if _, ok := r.Payload["squadId"]; !ok {
			http.Error(w, "no field squadId in payload", http.StatusBadRequest)
			return
		}
		if _, ok := r.Payload["password"]; !ok {
			http.Error(w, "no field password in payload", http.StatusBadRequest)
			return
		}
		if _, ok := r.Payload["squadName"]; !ok {
			http.Error(w, "no field squadName in payload", http.StatusBadRequest)
			return
		}
		if _, ok := r.Payload["squadType"]; !ok {
			http.Error(w, "no field squadType in payload", http.StatusBadRequest)
			return
		}
		if err = m.ModifySquad(r.Payload["squadId"], r.From, r.Payload["squadName"], SquadType(r.Payload["squadType"]), r.Payload["password"]); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})
	}
	return
}
